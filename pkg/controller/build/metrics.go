/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package build

import (
	"math"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus"

	v1 "github.com/apache/camel-k/pkg/apis/camel/v1"
)

const (
	buildResultLabel = "result"
	buildTypeLabel   = "type"
)

var (
	buildDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "camel_k_build_duration_seconds",
			Help: "Camel K build duration",
			Buckets: []float64{
				30 * time.Second.Seconds(),
				1 * time.Minute.Seconds(),
				1.5 * time.Minute.Seconds(),
				2 * time.Minute.Seconds(),
				5 * time.Minute.Seconds(),
				10 * time.Minute.Seconds(),
			},
		},
		[]string{
			buildResultLabel,
			buildTypeLabel,
		},
	)

	buildRecovery = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "camel_k_build_recovery_attempts",
			Help:    "Camel K build recovery attempts",
			Buckets: []float64{0, 1, 2, 3, 4, 5},
		},
		[]string{
			buildResultLabel,
			buildTypeLabel,
		},
	)

	queueDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "camel_k_build_queue_duration_seconds",
			Help: "Camel K build queue duration",
			Buckets: []float64{
				5 * time.Second.Seconds(),
				15 * time.Second.Seconds(),
				30 * time.Second.Seconds(),
				1 * time.Minute.Seconds(),
				5 * time.Minute.Seconds(),
			},
		},
		[]string{
			buildTypeLabel,
		},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(buildDuration, buildRecovery, queueDuration)
}

func observeBuildQueueDuration(build *v1.Build) {
	queueDuration.WithLabelValues(build.Labels[v1.IntegrationKitLayoutLabel]).
		Observe(time.Since(getBuildQueuingTime(build)).Seconds())
}

func observeBuildResult(build *v1.Build, phase v1.BuildPhase, duration time.Duration) {
	attempt, attemptMax := getBuildAttemptFor(build)

	if phase == v1.BuildPhaseFailed && attempt >= attemptMax {
		// The phase will be updated in the recovery action,
		// so let's account for it right now.
		phase = v1.BuildPhaseError
	}

	resultLabel := phase.String()
	typeLabel := build.Labels[v1.IntegrationKitLayoutLabel]

	buildRecovery.WithLabelValues(resultLabel, typeLabel).Observe(float64(attempt))
	buildDuration.WithLabelValues(resultLabel, typeLabel).Observe(duration.Seconds())
}

func getBuildAttemptFor(build *v1.Build) (int, int) {
	attempt := 0
	attemptMax := math.MaxInt32
	if failure := build.Status.Failure; failure != nil {
		attempt += failure.Recovery.Attempt
		attemptMax = failure.Recovery.AttemptMax
	}
	return attempt, attemptMax
}

func getBuildQueuingTime(build *v1.Build) time.Time {
	queuingTime := build.CreationTimestamp.Time
	if failure := build.Status.Failure; failure != nil {
		queuingTime = failure.Recovery.AttemptTime.Time
	}
	return queuingTime
}
