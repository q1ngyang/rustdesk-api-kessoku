package starrycontrol

import (
	"context"
	"testing"
)

func TestRequestMetadataRequiresExactlyOneIdentityKind(t *testing.T) {
	cases := []struct {
		name     string
		metadata RequestMetadata
		valid    bool
	}{
		{name: "actor", metadata: RequestMetadata{ActorUserID: 42, RequestID: "actor"}, valid: true},
		{name: "service", metadata: RequestMetadata{RequestID: "service", Service: true}, valid: true},
		{name: "ambiguous", metadata: RequestMetadata{ActorUserID: 42, RequestID: "ambiguous", Service: true}},
		{name: "anonymous", metadata: RequestMetadata{RequestID: "anonymous"}},
		{name: "missing request id", metadata: RequestMetadata{ActorUserID: 42}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, ok := MetadataFromContext(WithRequestMetadata(context.Background(), test.metadata))
			if ok != test.valid {
				t.Fatalf("metadata validity = %v, want %v", ok, test.valid)
			}
		})
	}
}
