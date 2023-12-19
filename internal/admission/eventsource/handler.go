package eventsource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	common "github.com/argoproj/argo-events/pkg/apis/common"
	esv1alpha1 "github.com/argoproj/argo-events/pkg/apis/eventsource/v1alpha1"
)

const defaultAnnotationKey string = "v1alpha1.argoslower.kanopy-platform/known-source"

type Handler struct {
	annotationKey string
	decoder       *admission.Decoder
}

func NewHandler(key string) *Handler {
	ak := defaultAnnotationKey
	if key != nil {
		ak = key
	}
	return &Handler{
		annotationKey: ak,
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

	if err := h.decoder.Decode(req, out); err != nil {
		log.Error(err, fmt.Sprintf("failed to decode eventsource request: %s", req.Name))
		return admission.Errored(http.StatusBadRequest, err)
	}

	if out.ObjectMeta.Annotations == nil {
		return admission.PatchResponseFromRaw(req.Object.Raw, bytes)
	}

	_, ok := out.ObjectMeta.Annotations[h.annotationKey]
	if !ok {
		return admission.PatchResponseFromRaw(req.Object.Raw, bytes)
	}

	out.Template = setIstioLabel(out.Template)

	bytes, err := json.Marshal(out)
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to marshal gateway: %s", out.Name))
		return admission.Errored(http.StatusInternalServerError, err)

	}
	return admission.PatchResponseFromRaw(req.Object.Raw, bytes)
}

func setIstioLabel(in *esv1alpha1.Template) *esv1alpha1.Template {
	out = in.DeepCopy()
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
