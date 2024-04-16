package codegen

type Serializable interface {
	int | int8 | int16 | int32 | int64 | bool |
		uint | uint8 | uint16 | uint32 | uint64 |
		float32 | float64 |
		complex64 | complex128 |
		string |
		[]byte
}

type Serialization interface {
	Serialize([]byte) error
	Deserialize([]byte) error
}
