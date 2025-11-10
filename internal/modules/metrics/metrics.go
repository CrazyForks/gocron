package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TaskExecutionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gocron_task_execution_total",
			Help: "Total number of task executions",
		},
		[]string{"task_id", "task_name", "status"},
	)

	TaskExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gocron_task_execution_duration_seconds",
			Help:    "Task execution duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"task_id", "task_name"},
	)

	ActiveTasks = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "gocron_active_tasks",
			Help: "Number of currently active tasks",
		},
	)

	TaskNodes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "gocron_task_nodes",
			Help: "Number of registered task nodes",
		},
	)
)
