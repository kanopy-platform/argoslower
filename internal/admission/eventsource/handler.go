package eventsource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	perrs "github.com/kanopy-platform/argoslower/pkg/errors"

	common "github.com/argoproj/argo-events/pkg/apis/common"
	esv1alpha1 "github.com/argoproj/argo-events/pkg/apis/eventsource/v1alpha1"
)

const DefaultAnnotationKey string = "v1alpha1.argoslower.kanopy-platform/known-source"

type Handler struct {
	annotationKey string
	meshChecker   MeshChecker
	decoder       *admission.Decoder
}

func NewHandler(mc MeshChecker) *Handler {
	return &Handler{
		annotationKey: DefaultAnnotationKey,
		meshChecker:   mc,
	}
}

func (h *Handler) SetAnnotationKey(key string) {
	if key != "" {
		h.annotationKey = key
	}
}

func (h *Handler) SetupWithManager(m manager.Manager) {
	m.GetWebhookServer().Register("/mutate/eventsource", &webhook.Admission{Handler: h})
}

func (h *Handler) InjectDecoder(decoder *admission.Decoder) error {
	if decoder == nil {
		return fmt.Errorf("decoder cannot be nil")
	}
	h.decoder = decoder
	return nil
}

func (h *Handler) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := log.FromContext(ctx)

	out := &esv1alpha1.EventSource{}

	log.V(1).Info("Looking for annotation")
	if err := h.decoder.Decode(req, out); err != nil {
		log.Error(err, fmt.Sprintf("failed to decode eventsource request: %s", req.Name))
		return admission.Errored(http.StatusBadRequest, err)
	}

	_, ok := out.Annotations[h.annotationKey]
	if !ok {
		log.V(1).Info("Annotation not found, ignoring eventsource")
		return admission.Allowed("No modifications needed")
	}

	onMesh, err := h.meshChecker.OnMesh(out.Namespace)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !onMesh {
		return admission.Denied(fmt.Sprintf("Namespace %s is not opted into the mesh. Please contact your cluster administrator and try again", out.Namespace))
	}

	err = ValidateEventSource(out)
	if err != nil {
		return admission.Denied(err.Error())
	}

	out.Spec.Template = setIstioLabel(out.Spec.Template)

	bytes, err := json.Marshal(out)
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to marshal eventsource: %s/%s", out.Namespace, out.Name))
		return admission.Errored(http.StatusInternalServerError, err)

	}
	return admission.PatchResponseFromRaw(req.Object.Raw, bytes)
}

func setIstioLabel(in *esv1alpha1.Template) *esv1alpha1.Template {
	out := in.DeepCopy()
	if out == nil {
		out = &esv1alpha1.Template{}
	}

	if out.Metadata == nil {
		out.Metadata = &common.Metadata{}
	}

	if out.Metadata.Labels == nil {
		out.Metadata.Labels = map[string]string{}

	}

	out.Metadata.Labels["sidecar.istio.io/inject"] = "true"

	return out
}

type MeshChecker interface {
	OnMesh(namespace string) (bool, error)
}

func ValidateEventSource(es *esv1alpha1.EventSource) error {

	if len(es.Spec.Webhook) == 0 && len(es.Spec.Github) == 0 {
		return perrs.NewUnretryableError(fmt.Errorf("EventSource %s/%s has no supported webhook configuration", es.Namespace, es.Name))
	}

	if es.Spec.Webhook != nil {
		var err error

		for hook, spec := range es.Spec.Webhook {
			e := validateWebhookEventSource(&spec)
			if e != nil {
				err = perrs.NewUnretryableError(errors.Join(err, fmt.Errorf("Webhook %s misconfigured: %w", hook, e)))
			}
		}

		if err != nil {
			return err
		}
	}

	if es.Spec.Github != nil {
		var err error

		for hook, spec := range es.Spec.Github {
			e := validateGithubEventSource(&spec)
			if e != nil {
				err = perrs.NewUnretryableError(errors.Join(err, fmt.Errorf("Github webhook %s misconfigured: %w", hook, e)))
			}
		}

		if err != nil {
			return err
		}

	}

	return nil
}

func validateWebhookEventSource(spec *esv1alpha1.WebhookEventSource) error {
	// This is Bearer token authentication provided by argo-events
	if spec.AuthSecret == nil {
		return perrs.NewUnretryableError(fmt.Errorf("Webhook EventSources require auth tokens. Ensure an authSecret secret selector configured."))
	}

	return nil
}

func validateGithubEventSource(spec *esv1alpha1.GithubEventSource) error {
	// Github webhooks provide signed messages for validation of the message payload.
	// verification is implemented by argo-events
	// https://github.com/argoproj/argo-events/blob/e948d7337aec619dd48fb2a065126b025ed9281d/eventsources/sources/github/start.go#L383
	if spec.WebhookSecret == nil {
		return perrs.NewUnretryableError(fmt.Errorf("Github webhook EventSources require HMAC signing validation for ingress. Ensure a webhookSecret secret selector is provided."))
	}

	return nil
}
