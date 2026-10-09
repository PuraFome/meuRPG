package campaignpackage_test

import "google.golang.org/protobuf/encoding/protojson"

import "google.golang.org/protobuf/proto"

func protojsonUnmarshal(b []byte, m proto.Message) error {
	return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(b, m)
}
