package starrycontrol

import "context"

type requestMetadataKey struct{}

type RequestMetadata struct {
	ActorUserID uint
	RequestID   string
}

func WithRequestMetadata(ctx context.Context, metadata RequestMetadata) context.Context {
	return context.WithValue(ctx, requestMetadataKey{}, metadata)
}

func MetadataFromContext(ctx context.Context) (RequestMetadata, bool) {
	metadata, ok := ctx.Value(requestMetadataKey{}).(RequestMetadata)
	return metadata, ok && metadata.ActorUserID > 0 && metadata.RequestID != ""
}
