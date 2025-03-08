package types

type Tag struct {
	Group   uint16
	Element uint16
	VR      string
	Length  uint32
	Value   []byte
}
