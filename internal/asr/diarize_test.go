package asr

import (
	"math"
	"testing"
)

func TestClusterEmbeddingsUnusableDistancesStayBounded(t *testing.T) {
	vecs := make([][]float64, 171)
	for i := range vecs {
		vecs[i] = []float64{math.NaN()}
	}
	ids := clusterEmbeddings(vecs)
	if uniqueCount(ids) > maxSpeakers {
		t.Fatalf("unusable distances produced %d speakers", uniqueCount(ids))
	}
	for i, id := range ids {
		if id < 1 || id > maxSpeakers {
			t.Fatalf("segment %d has invalid speaker %d", i, id)
		}
	}
}
