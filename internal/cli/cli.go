package cli

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	esadd "github.com/kanopy-platform/argoslower/internal/admission/eventsource"
	sadd "github.com/kanopy-platform/argoslower/internal/admission/sensor"
	esctrl "github.com/kanopy-platform/argoslower/internal/controllers/eventsource"
	ic "github.com/kanopy-platform/argoslower/pkg/ingress/v1/istio"
	"github.com/kanopy-platform/argoslower/pkg/iplister"
	ghc "github.com/kanopy-platform/argoslower/pkg/iplister/clients/github"
	filedecoder "github.com/kanopy-platform/argoslower/pkg/iplister/decoder/file"
	"github.com/kanopy-platform/argoslower/pkg/iplister/decoder/officeips"
	"github.com/kanopy-platform/argoslower/pkg/iplister/reader/file"
	"github.com/kanopy-platform/argoslower/pkg/iplister/reader/http"
	"github.com/kanopy-platform/argoslower/pkg/namespace"
	"github.com/kanopy-platform/argoslower/pkg/ratelimit"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
	k8szap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/source"

	sensor "github.com/argoproj/argo-events/pkg/apis/sensor/v1alpha1"

	eventscommon "github.com/argoproj/argo-events/common"
	eventsource "github.com/argoproj/argo-events/pkg/apis/eventsource/v1alpha1"
	esclient "github.com/argoproj/argo-events/pkg/client/eventsource/clientset/versioned"
	esinformerv1alpha1 "github.com/argoproj/argo-events/pkg/client/eventsource/informers/externalversions"

	istioclient "istio.io/client-go/pkg/clientset/versioned"
	istioinformer "istio.io/client-go/pkg/informers/externalversions"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("argoslower")
)

func setupScheme() {
	utilruntime.Must(sensor.AddToScheme(scheme))
	utilruntime.Must(eventsource.AddToScheme(scheme))
}

type RootCommand struct {
	k8sFlags *genericclioptions.ConfigFlags
}

func NewRootCommand() *cobra.Command {
	k8sFlags := genericclioptions.NewConfigFlags(true)

	root := &RootCommand{k8sFlags}

	cmd := &cobra.Command{
		Use:               "argoslower",
		PersistentPreRunE: root.persistentPreRunE,
		RunE:              root.runE,
	}

	cmd.PersistentFlags().String("log-level", "info", "Configure log level")
	cmd.PersistentFlags().Int("webhook-listen-port", 8443, "Admission webhook listen port")
	cmd.PersistentFlags().Int("metrics-listen-port", 8081, "Metrics listen port")
	cmd.PersistentFlags().String("webhook-certs-dir", "/etc/webhook/certs", "Admission webhook TLS certificate directory")
	cmd.PersistentFlags().Bool("dry-run", false, "Controller dry-run changes only")
	cmd.PersistentFlags().String("default-rate-limit-unit", "Second", "Default rate limit unit")
	cmd.PersistentFlags().Int32("default-requests-per-unit", 1, "Default requests per unit")
	cmd.PersistentFlags().String("rate-limit-unit-annotation", "kanopy-events/rate-limit-unit", "Namespace annotation for rate limit unit")
	cmd.PersistentFlags().String("requests-per-unit-annotation", "kanopy-events/requests-per-unit", "Namespace annotation for requests per unit")
	cmd.PersistentFlags().Bool("enable-webhook-controller", false, "Enable webhook controller")
	cmd.PersistentFlags().String("webhook-url", "webhooks.example.com", "Base url assocated with webhooks")
	cmd.PersistentFlags().String("admin-namespace", "routing", "Ingress controller admin namespace")
	cmd.PersistentFlags().String("gateway-namespace", "routing-rules", "Namespace of the ingress gateway")
	cmd.PersistentFlags().String("gateway-name", "argo-webhook-gateway", "Name of the ingress gateway")
	cmd.PersistentFlags().String("gateway-selector", "istio=istio-ingressgateway-public", "Label selector for the ingress gateway as a key=value comma delimited string")
	cmd.PersistentFlags().String("supported-hooks", "github=github", "comma separated key=value list used for assigning IPGetters for various hook annotations")

	k8sFlags.AddFlags(cmd.PersistentFlags())
	// no need to check err, this only checks if variadic args != 0
	_ = viper.BindEnv("kubeconfig", "KUBECONFIG")

	//	cmd.AddCommand(newVersionCommand())
	return cmd
}

func (c *RootCommand) persistentPreRunE(cmd *cobra.Command, args []string) error {
	// bind flags to viper
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.SetEnvPrefix("app")
	viper.AutomaticEnv()

	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return err
	}

	// set log level
	logLevel, err := zapcore.ParseLevel(viper.GetString("log-level"))
	if err != nil {
		return err
	}

	ctrl.SetLogger(k8szap.New(
		k8szap.Level(logLevel),
		k8szap.RawZapOpts(
			zap.Fields(zap.Bool("dry-run", viper.GetBool("dry-run"))),
		)),
	)

	return nil
}

func (c *RootCommand) runE(cmd *cobra.Command, args []string) error {
	dryRun := viper.GetBool("dry-run")
	if dryRun {
		klog.Log.Info("running in dry-run mode.")
	}

	cfg, err := c.k8sFlags.ToRESTConfig()
	if err != nil {
		return err
	}

	ctx := signals.SetupSignalHandler()

	setupScheme()

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                     scheme,
		Host:                       "0.0.0.0",
		Port:                       viper.GetInt("webhook-listen-port"),
		CertDir:                    viper.GetString("webhook-certs-dir"),
		MetricsBindAddress:         fmt.Sprintf("0.0.0.0:%d", viper.GetInt("metrics-listen-port")),
		HealthProbeBindAddress:     ":8080",
		LeaderElection:             true,
		LeaderElectionID:           "argoslower",
		LeaderElectionResourceLock: "leases",
		DryRunClient:               dryRun,
	})

	if err != nil {
		klog.Log.Error(err, "unable to set up controller manager")
		return err
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		return err
	}

	esc := esclient.NewForConfigOrDie(cfg)
	esinformerFactory := esinformerv1alpha1.NewSharedInformerFactoryWithOptions(esc, 1*time.Minute)

	esi := esinformerFactory.Argoproj().V1alpha1().EventSources()

	k8sClientSet := kubernetes.NewForConfigOrDie(cfg)
	k8sInformerFactory := informers.NewSharedInformerFactoryWithOptions(k8sClientSet, 1*time.Minute)

	namespacesInformer := k8sInformerFactory.Core().V1().Namespaces()
	namespacesInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(new interface{}) {},
	})

	k8sInformerFactory.Start(wait.NeverStop)
	k8sInformerFactory.WaitForCacheSync(wait.NeverStop)

	rlua := viper.GetString("rate-limit-unit-annotation")
	rlra := viper.GetString("requests-per-unit-annotation")

	nsInformer := namespace.NewNamespaceInfo(namespacesInformer.Lister(), rlua, rlra)

	drlu := viper.GetString("default-rate-limit-unit")
	drlr := viper.GetInt32("default-requests-per-unit")
	rlc := ratelimit.NewRateLimitCalculatorOrDie(drlu, drlr)

	if viper.GetBool("enable-webhook-controller") {
		//creater a filtered informer for resources with the event-source labal
		selector := eventscommon.LabelEventSourceName
		_, err = labels.Parse(selector)
		if err != nil {
			return err
		}

		labelOptions := informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = selector
		})

		filteredk8sInformerFactory := informers.NewSharedInformerFactoryWithOptions(k8sClientSet, 1*time.Minute, labelOptions)

		filteredServiceInfomer := filteredk8sInformerFactory.Core().V1().Services()
		filteredServiceInfomer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(new interface{}) {},
		})

		filteredk8sInformerFactory.Start(wait.NeverStop)
		filteredk8sInformerFactory.WaitForCacheSync(wait.NeverStop)

		istioCS := istioclient.NewForConfigOrDie(cfg)
		istioOptions := istioinformer.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = selector
		})
		filteredIstioInformerFactory := istioinformer.NewSharedInformerFactoryWithOptions(istioCS, 1*time.Minute, istioOptions)

		filteredVirtualServiceInfomer := filteredIstioInformerFactory.Networking().V1beta1().VirtualServices().Informer()
		filteredVirtualServiceInfomer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(new interface{}) {},
		})

		filteredAuthorizationPolicyInformer := filteredIstioInformerFactory.Security().V1beta1().AuthorizationPolicies().Informer()
		filteredAuthorizationPolicyInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(new interface{}) {},
		})

		filteredIstioInformerFactory.Start(wait.NeverStop)
		filteredIstioInformerFactory.WaitForCacheSync(wait.NeverStop)

		sadd.NewHandler(nsInformer, rlc).SetupWithManager(mgr)
		esadd.NewHandler(nsInformer).SetupWithManager(mgr)

		gws := stringToMap(viper.GetString("gateway-selector"), ",", "=")
		if len(gws) == 0 {
			return fmt.Errorf("Invalid gateway-selector: %s", viper.GetString("gateway-selector"))
		}

		ingressClient := ic.NewClient(istioCS, gws)
		escc := esctrl.NewEventSourceIngressControllerConfig()
		escc.Gateway = types.NamespacedName{
			Namespace: viper.GetString("gateway-namespace"),
			Name:      viper.GetString("gateway-name"),
		}

		escc.BaseURL = viper.GetString("webhook-url")
		escc.AdminNamespace = viper.GetString("admin-namespace")

		esController := esctrl.NewEventSourceIngressController(esi.Lister(), filteredServiceInfomer.Lister(), escc, ingressClient)

		hookConfig := stringToMap(viper.GetString("supported-hooks"), ",", "=")

		githubGetter := ghc.New()

		for hook, provider := range hookConfig {
			if hook == "" {
				continue
			}
			switch provider {
			case "github":
				esController.SetIPGetter(hook, githubGetter)
			case "file":
				f := file.New(viper.GetString("IPFILE"))
				ips := viper.GetString("IPFILE_SOURCES")
				d := filedecoder.New(strings.Split(ips, ",")...)
				g := iplister.New(f, d)
				esController.SetIPGetter(hook, g)

			case "officeips":
				h := http.New(viper.GetString("OFFICEIP_URL"), http.WithBasicAuth(viper.GetString("OFFICEIP_USER"), viper.GetString("OFFICEIP_PASSWORD")))
				d := officeips.New()
				g := iplister.New(h, d)
				esController.SetIPGetter(hook, g)
			case "any":
				klog.Log.V(1).Info(fmt.Sprintf("The any provider is only designed for debug and testing use. Configuring for hook type: %s", hook))
				g := &iplister.AnyGetter{}
				esController.SetIPGetter(hook, g)

			default:
				err := fmt.Errorf("Unkonwn webhook provider type %s", provider)
				klog.Log.Error(err, err.Error())
				return err
			}
		}

		esi.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(new interface{}) {}})

		esinformerFactory.Start(wait.NeverStop)
		esinformerFactory.WaitForCacheSync(wait.NeverStop)

		ctrl, err := controller.New("argoslower-eventsource-controller", mgr, controller.Options{
			Reconciler: esController,
		})

		if err != nil {
			return err
		}

		if e := ctrl.Watch(&source.Informer{Informer: esi.Informer()}, &handler.EnqueueRequestForObject{}); e != nil {
			return e
		}
	}

	return mgr.Start(ctx)
}

func stringToMap(in, delim, split string) map[string]string {
	out := map[string]string{}
	for _, v := range strings.Split(in, delim) {
		mark := strings.Index(v, split)
		if mark < 0 || mark+1 >= len(v) {
			continue
		}
		out[v[:mark]] = v[mark+1:]
	}
	return out
}
