package remote

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type jsonSerializer struct {
	marshalOptions   protojson.MarshalOptions
	unmarshalOptions protojson.UnmarshalOptions
}

func newJSONSerializer() Serializer {
	return &jsonSerializer{
		marshalOptions:   protojson.MarshalOptions{},
		unmarshalOptions: protojson.UnmarshalOptions{DiscardUnknown: true},
	}
}

func (j *jsonSerializer) Serialize(msg interface{}) ([]byte, error) {
	if message, ok := msg.(*JSONMessage); ok {
		return []byte(message.JSON), nil
	} else if message, ok := msg.(proto.Message); ok {

		bytes, err := j.marshalOptions.Marshal(message)
		if err != nil {
			return nil, err
		}

		return bytes, nil
	}
	return nil, fmt.Errorf("msg must be proto.Message")
}

func (j *jsonSerializer) Deserialize(typeName string, b []byte) (interface{}, error) {
	mt, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(typeName))
	if err != nil {
		m := &JSONMessage{
			TypeName: typeName,
			JSON:     string(b),
		}
		return m, nil
	}

	instance := mt.New().Interface()
	if err := j.unmarshalOptions.Unmarshal(b, instance); err != nil {
		return nil, err
	}

	return instance, nil

}

func (j *jsonSerializer) GetTypeName(msg interface{}) (string, error) {
	if message, ok := msg.(*JSONMessage); ok {
		return message.TypeName, nil
	} else if message, ok := msg.(proto.Message); ok {
		typeName := proto.MessageName(message)

		return string(typeName), nil
	}

	return "", fmt.Errorf("msg must be proto.Message")
}
