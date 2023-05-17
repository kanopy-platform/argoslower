package admission

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	sensorv1alpha1 "github.com/argoproj/argo-events/pkg/apis/sensor/v1alpha1"
	"github.com/kanopy-platform/argoslower/pkg/ratelimit"
)

type Handler struct {
	rlg     RateLimitGetter
	drlc    *ratelimit.RateLimitCalculator
	decoder *admission.Decoder
}

func NewHandler(rlg RateLimitGetter, drlc *ratelimit.RateLimitCalculator) *Handler {
	return &Handler{
		rlg:  rlg,
		drlc: drlc,
	}
}

func (h *Handler) SetupWithManager(m manager.Manager) {
	m.GetWebhookServer().Register("/mutate", &webhook.Admission{Handler: h})
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

	out := &sensorv1alpha1.Sensor{}

	if err := h.decoder.Decode(req, out); err != nil {
		log.Error(err, fmt.Sprintf("failed to decode sensor request: %s", req.Name))
		return admission.Errored(http.StatusBadRequest, err)
	}

	defaultRate, err := h.rlg.RateLimit(out.Namespace)
	if err != nil {
		log.Error(err, fmt.Sprintf("Cannot determine default ratelimit for namespace: %s", out.Namespace))
		return admission.Errored(http.StatusBadRequest, err)
	}

	ts := []sensorv1alpha1.Trigger{}
	for _, trigger := range out.Spec.Triggers {
		if trigger.Template != nil && trigger.Template.K8s != nil {
			rate := h.drlc.Calculate(defaultRate, trigger.RateLimit)
			trigger.RateLimit = &rate
		}

		ts = append(ts, trigger)
	}

	out.Spec.Triggers = ts

	jsonSensor, err := json.Marshal(out)
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to marshal gateway: %s", out.Name))
		return admission.Errored(http.StatusInternalServerError, err)

	}
	return admission.PatchResponseFromRaw(req.Object.Raw, jsonSensor)

}
