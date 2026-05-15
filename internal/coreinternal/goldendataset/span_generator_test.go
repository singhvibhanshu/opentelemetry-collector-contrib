// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/internal/metadata"
)

func TestGenerateParentSpan(t *testing.T) {
	random := rand.Reader
	traceID := generateTraceID(random)
	spanInputs := &PICTSpanInputs{
		Parent:     SpanParentRoot,
		Tracestate: TraceStateEmpty,
		Kind:       SpanKindServer,
		Attributes: SpanAttrHTTPServer,
		Events:     SpanChildCountTwo,
		Links:      SpanChildCountOne,
		Status:     SpanStatusOk,
	}
	span := ptrace.NewSpan()
	fillSpan(traceID, pcommon.SpanID([8]byte{}), "/gotest-parent", spanInputs, random, span)
	assert.Equal(t, traceID, span.TraceID())
	assert.True(t, span.ParentSpanID().IsEmpty())
	assert.Equal(t, 11, span.Attributes().Len())
	assert.Equal(t, ptrace.StatusCodeOk, span.Status().Code())
}

func TestGenerateChildSpan(t *testing.T) {
	random := rand.Reader
	traceID := generateTraceID(random)
	parentID := generateSpanID(random)
	spanInputs := &PICTSpanInputs{
		Parent:     SpanParentChild,
		Tracestate: TraceStateEmpty,
		Kind:       SpanKindClient,
		Attributes: SpanAttrDatabaseSQL,
		Events:     SpanChildCountEmpty,
		Links:      SpanChildCountEmpty,
		Status:     SpanStatusOk,
	}
	span := ptrace.NewSpan()
	fillSpan(traceID, parentID, "get_test_info", spanInputs, random, span)
	assert.Equal(t, traceID, span.TraceID())
	assert.Equal(t, parentID, span.ParentSpanID())
	assert.Equal(t, 12, span.Attributes().Len())
	assert.Equal(t, ptrace.StatusCodeOk, span.Status().Code())
}

func TestGenerateSpans(t *testing.T) {
	random := rand.Reader
	count1 := 16
	spans := ptrace.NewSpanSlice()
	err := appendSpans(count1, "testdata/generated_pict_pairs_spans.txt", random, spans)
	assert.NoError(t, err)
	assert.Equal(t, count1, spans.Len())

	count2 := 256
	spans = ptrace.NewSpanSlice()
	err = appendSpans(count2, "testdata/generated_pict_pairs_spans.txt", random, spans)
	assert.NoError(t, err)
	assert.Equal(t, count2, spans.Len())

	count3 := 118
	spans = ptrace.NewSpanSlice()
	err = appendSpans(count3, "testdata/generated_pict_pairs_spans.txt", random, spans)
	assert.NoError(t, err)
	assert.Equal(t, count3, spans.Len())
}

func TestHTTPClientIPMigrationWithNetworkFeatureGates(t *testing.T) {
	oldDontEmitV0 := metadata.InternalCoreinternalGoldendatasetDontEmitV0NetworkConventionsFeatureGate.IsEnabled()
	oldEmitV1 := metadata.InternalCoreinternalGoldendatasetEmitV1NetworkConventionsFeatureGate.IsEnabled()
	t.Cleanup(func() {
		assert.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0NetworkConventionsFeatureGate.ID(), oldDontEmitV0))
		assert.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1NetworkConventionsFeatureGate.ID(), oldEmitV1))
	})

	testCases := []struct {
		name         string
		dontEmitV0   bool
		emitV1       bool
		expectV0Attr bool
		expectV1Attr bool
	}{
		{
			name:         "v0 only",
			dontEmitV0:   false,
			emitV1:       false,
			expectV0Attr: true,
			expectV1Attr: false,
		},
		{
			name:         "double publish",
			dontEmitV0:   false,
			emitV1:       true,
			expectV0Attr: true,
			expectV1Attr: true,
		},
		{
			name:         "v1 only",
			dontEmitV0:   true,
			emitV1:       true,
			expectV0Attr: false,
			expectV1Attr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0NetworkConventionsFeatureGate.ID(), tc.dontEmitV0))
			assert.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1NetworkConventionsFeatureGate.ID(), tc.emitV1))

			httpServerAttrs := pcommon.NewMap()
			appendHTTPServerAttributes(true, httpServerAttrs)
			_, hasV0HTTPServerAttr := httpServerAttrs.Get("http.client_ip")
			_, hasV1HTTPServerAttr := httpServerAttrs.Get("client.address")
			assert.Equal(t, tc.expectV0Attr, hasV0HTTPServerAttr)
			assert.Equal(t, tc.expectV1Attr, hasV1HTTPServerAttr)

			maxCountAttrs := pcommon.NewMap()
			appendMaxCountAttributes(true, maxCountAttrs)
			_, hasV0MaxCountAttr := maxCountAttrs.Get("http.client_ip")
			_, hasV1MaxCountAttr := maxCountAttrs.Get("client.address")
			assert.Equal(t, tc.expectV0Attr, hasV0MaxCountAttr)
			assert.Equal(t, tc.expectV1Attr, hasV1MaxCountAttr)
		})
	}
}
