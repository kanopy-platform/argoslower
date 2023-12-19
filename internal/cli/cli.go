package cli

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	esadd "github.com/kanopy-platform/argoslower/internal/admission/eventsource"
	sadd "github.com/kanopy-platform/argoslower/internal/admission/sensor"
	"github.com/kanopy-platform/argoslower/pkg/namespace"
	"github.com/kanopy-platform/argoslower/pkg/ratelimit"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/healthz"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
	k8szap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	sensor "github.com/argoproj/argo-events/pkg/apis/sensor/v1alpha1"
	sensorclient "github.com/argoproj/argo-events/pkg/client/sensor/clientset/versioned"
	sinformerv1alpha1 "github.com/argoproj/argo-events/pkg/client/sensor/informers/externalversions"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("argoslower")
)

func setupScheme() {
	utilruntime.Must(sensor.AddToScheme(scheme))
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

	sclient := sensorclient.NewForConfigOrDie(cfg)
	sinformerFactory := sinformerv1alpha1.NewSharedInformerFactoryWithOptions(sclient, 1*time.Minute)
	sinformerFactory.Start(wait.NeverStop)
	sinformerFactory.WaitForCacheSync(wait.NeverStop)

	sensorInformer := sinformerFactory.Argoproj().V1alpha1().Sensors()
	sensorInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(new interface{}) {},
	})

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

	sadd.NewHandler(nsInformer, rlc).SetupWithManager(mgr)
	esadd.NewHandler().SetupWithmanager(mgr)

	return mgr.Start(ctx)
}
