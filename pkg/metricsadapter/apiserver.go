package metricsadapter

// MetricsServer is a metrics server
type MetricsServer struct {
	metricsController *MetricsController
	metricsAdapter    *MetricsAdapter
}

// NewMetricsServer creates a new metrics server
func NewMetricsServer(controller *MetricsController, metricsAdapter *MetricsAdapter) *MetricsServer {
	return &MetricsServer{
		metricsController: controller,
		metricsAdapter:    metricsAdapter,
	}
}

// StartServer starts the metrics server
func (m *MetricsServer) StartServer(stopCh <-chan struct{}) error {
	go m.metricsController.startController(stopCh)
	return m.metricsAdapter.Run(stopCh)
}
