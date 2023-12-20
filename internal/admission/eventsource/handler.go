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
	meshChecker   MeshChecker
	decoder       *admission.Decoder
}

func NewHandler(mc MeshChecker) *Handler {
	return &Handler{
		annotationKey: defaultAnnotationKey,
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

	log.Info(fmt.Sprintf("Received request: %v", req))

	out := &esv1alpha1.EventSource{}

	log.Info("Looking for annotation")
	if err := h.decoder.Decode(req, out); err != nil {
		log.Error(err, fmt.Sprintf("failed to decode eventsource request: %s", req.Name))
		return admission.Errored(http.StatusBadRequest, err)
	}

	_, ok := out.Annotations[h.annotationKey]
	if !ok {
		log.Info("Annotation not found, ignoring eventsource")
		return admission.Allowed("No modifications needed")
	}

	onMesh, err := h.meshChecker.OnMesh(out.Namespace)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !onMesh {
		return admission.Denied(fmt.Sprintf("Namespace %s is not opted into the mesh. Please contact your cluster administrator and try again", out.Namespace))
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
