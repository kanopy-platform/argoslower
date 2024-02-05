package admission

import (
	"context"
	"fmt"

	aes "github.com/argoproj/argo-events/pkg/apis/eventsource"
	as "github.com/argoproj/argo-events/pkg/apis/sensor"
	event "github.com/kanopy-platform/argoslower/internal/admission/eventsource"
	sensor "github.com/kanopy-platform/argoslower/internal/admission/sensor"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type RoutingHandler struct {
	sensorHandler      *sensor.Handler
	eventSourceHandler *event.Handler
}

func NewRoutingHandler(sh *sensor.Handler, es *event.Handler) *RoutingHandler {
	return &RoutingHandler{
		sensorHandler:      sh,
		eventSourceHandler: es,
	}
}

func (h *RoutingHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	kind := req.RequestKind
	if kind == nil {
		kind = &req.Kind
	}

	switch {
	case kind.Kind == aes.Kind && h.eventSourceHandler != nil:
		return h.eventSourceHandler.Handle(ctx, req)
	case kind.Kind == as.Kind && h.sensorHandler != nil:
		return h.sensorHandler.Handle(ctx, req)
	default:
		return admission.Denied(fmt.Sprintf("Kind %s not supported by controller", kind.Kind))
	}

}

func (h *RoutingHandler) InjectDecoder(decoder *admission.Decoder) error {
	if h.sensorHandler != nil {
		err := h.sensorHandler.InjectDecoder(decoder)
		if err != nil {
			return err
		}
	}
	if h.eventSourceHandler != nil {
		err := h.eventSourceHandler.InjectDecoder(decoder)
		if err != nil {
			return err
		}

	}

	return nil
}

func (h *RoutingHandler) SetupWithManager(m manager.Manager) {
	m.GetWebhookServer().Register("/mutate", &webhook.Admission{Handler: h})
}
