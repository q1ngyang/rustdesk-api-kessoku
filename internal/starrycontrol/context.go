package starrycontrol

import "context"

type requestMetadataKey struct{}

type RequestMetadata struct {
	ActorUserID uint
	RequestID   string
	// Service is used only for non-user background operations such as checking
	// a public client report against Starry's private peer registry.
	Service bool
}

func WithRequestMetadata(ctx context.Context, metadata RequestMetadata) context.Context {
	return context.WithValue(ctx, requestMetadataKey{}, metadata)
}

func MetadataFromContext(ctx context.Context) (RequestMetadata, bool) {
	metadata, ok := ctx.Value(requestMetadataKey{}).(RequestMetadata)
	hasActor := metadata.ActorUserID > 0 && !metadata.Service
	isService := metadata.ActorUserID == 0 && metadata.Service
	return metadata, ok && metadata.RequestID != "" && (hasActor || isService)
}
