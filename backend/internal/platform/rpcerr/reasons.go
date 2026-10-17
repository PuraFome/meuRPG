package rpcerr

// Reason is the machine word in the JSON body of an HTTP route's error
// (`{"code": "...", "reason": "...", "message": "..."}`). The routes that are
// not Connect calls (uploads, downloads, the voice of a line) cannot carry a
// typed detail, so this word is how the app knows which Portuguese sentence to
// show. A reason is added here, to proto/error-reasons.json and to
// web/src/app/core/errors/body-reasons.ts together; archtest and the web spec
// fail when one of the three lacks it (docs/architecture/api.md#error-reasons).
type Reason string

// String is the reason as it goes on the wire.
func (r Reason) String() string { return string(r) }

const (
	// ReasonRateLimited: the caller asked too often; Retry-After says when to ask again.
	ReasonRateLimited Reason = "RATE_LIMITED"
	// ReasonBusy: the server is busy with other work and a retry may work.
	ReasonBusy Reason = "BUSY"
	// ReasonUnsupportedType: the file is not an image of a kind the server takes.
	ReasonUnsupportedType Reason = "UNSUPPORTED_TYPE"
	// ReasonTooLarge: the file is over the size limit.
	ReasonTooLarge Reason = "TOO_LARGE"
	// ReasonDimensions: the image has more pixels than the server reads.
	ReasonDimensions Reason = "DIMENSIONS"
	// ReasonCorrupt: the file cannot be read.
	ReasonCorrupt Reason = "CORRUPT"
	// ReasonMalformedRequest: the request is not shaped as the route expects.
	ReasonMalformedRequest Reason = "MALFORMED_REQUEST"
	// ReasonQuota: the campaign's gallery is full.
	ReasonQuota Reason = "QUOTA"
	// ReasonImagesOff: the server has no store for files, so it takes no uploads.
	ReasonImagesOff Reason = "IMAGES_OFF"
	// ReasonWrongLength: a part of an upload is not the length the upload said.
	ReasonWrongLength Reason = "WRONG_LENGTH"
	// ReasonOff: the feature is switched off on this server.
	ReasonOff Reason = "OFF"
	// ReasonDailyLimitReached: the server made all it may make today.
	ReasonDailyLimitReached Reason = "DAILY_LIMIT_REACHED"
	// ReasonUnavailable: the service behind the route failed; a retry may work.
	ReasonUnavailable Reason = "UNAVAILABLE"
	// ReasonRefused: the service behind the route refused the content.
	ReasonRefused Reason = "REFUSED"
)
