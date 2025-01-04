package protocol

import (
	"encoding/json"
	"fmt"

	"github.com/anycable/anycable-go/common"

	pb "github.com/anycable/anycable-go/protos"
)

// NewConnectMessage builds a connect RPC payload from the session env
func NewConnectMessage(env *common.SessionEnv) *pb.ConnectionRequest {
	return &pb.ConnectionRequest{
		Env: buildEnv(env),
	}
}

// NewCommandMessage builds a command RPC payload from the session env, command and channel names,
// and connection identifiers
func NewCommandMessage(env *common.SessionEnv, command string, channel string, identifiers string, data string) *pb.CommandMessage {
	msg := pb.CommandMessage{
		Command:               command,
		Env:                   buildChannelEnv(channel, env),
		Identifier:            channel,
		ConnectionIdentifiers: identifiers,
	}

	if data != "" {
		msg.Data = data
	}

	return &msg
}

// NewDisconnectMessage builds a disconnect RPC payload from the session env, connection identifiers and
// subscriptions
func NewDisconnectMessage(env *common.SessionEnv, identifiers string, subscriptions []string) *pb.DisconnectRequest {
	return &pb.DisconnectRequest{
		Identifiers:   identifiers,
		Subscriptions: subscriptions,
		Env:           buildDisconnectEnv(env),
	}
}

// ParseConnectResponse takes protobuf ConnectionResponse struct and converts into common.ConnectResult and/or error
func ParseConnectResponse(response *pb.ConnectionResponse) (*common.ConnectResult, error) {
	reply := common.ConnectResult{Transmissions: response.Transmissions}

	if response.Env != nil {
		reply.CState = response.Env.Cstate
	}

	if response.Status.String() == "SUCCESS" {
		reply.Identifier = response.Identifiers
		reply.Status = common.SUCCESS
		return &reply, nil
	}

	if response.Status.String() == "FAILURE" {
		reply.Status = common.FAILURE
		return &reply, nil
	}

	reply.Status = common.ERROR
	return &reply, fmt.Errorf("Application error: %s", response.ErrorMsg)
}

// ParseCommandResponse takes protobuf CommandResponse struct and converts into common.CommandResult and/or error
func ParseCommandResponse(response *pb.CommandResponse) (*common.CommandResult, error) {
	res := &common.CommandResult{
		Disconnect:     response.Disconnect,
		StopAllStreams: response.StopStreams,
		Streams:        response.Streams,
		StoppedStreams: response.StoppedStreams,
		Transmissions:  response.Transmissions,
	}

	if response.Env != nil {
		res.CState = response.Env.Cstate
		res.IState = response.Env.Istate
	}

	if response.Presence != nil {
		res.Presence = &common.PresenceEvent{
			Type: response.Presence.Type,
			ID:   response.Presence.Id,
		}

		if response.Presence.Info != "" {
			var info interface{}

			err := json.Unmarshal([]byte(response.Presence.Info), &info)

			if err != nil {
				res.Presence.Info = response.Presence.Info
			} else {
				res.Presence.Info = info
			}
		}
	}

	if response.Status.String() == "SUCCESS" {
		res.Status = common.SUCCESS
		return res, nil
	}

	if response.Status.String() == "FAILURE" {
		res.Status = common.FAILURE
		return res, nil
	}

	res.Status = common.ERROR
	return res, fmt.Errorf("Application error: %s", response.ErrorMsg)
}

// ParseDisconnectResponse takes protobuf DisconnectResponse struct and return error if any
func ParseDisconnectResponse(response *pb.DisconnectResponse) error {
	if response.Status.String() != "ERROR" {
		return nil
	}

	return fmt.Errorf("Application error: %s", response.ErrorMsg)
}

func buildEnv(env *common.SessionEnv) *pb.Env {
	protoEnv := pb.Env{Url: env.URL, Headers: *env.Headers}
	if env.ConnectionState != nil {
		protoEnv.Cstate = *env.ConnectionState
	}
	return &protoEnv
}

func buildDisconnectEnv(env *common.SessionEnv) *pb.Env {
	protoEnv := *buildEnv(env)

	if env.ChannelStates == nil {
		return &protoEnv
	}

	states := make(map[string]string)

	for id, state := range *env.ChannelStates {
		encodedState, _ := json.Marshal(state)

		states[id] = string(encodedState)
	}

	protoEnv.Istate = states

	return &protoEnv
}

func buildChannelEnv(id string, env *common.SessionEnv) *pb.Env {
	protoEnv := *buildEnv(env)

	if env.ChannelStates == nil {
		return &protoEnv
	}

	if _, ok := (*env.ChannelStates)[id]; ok {
		protoEnv.Istate = (*env.ChannelStates)[id]
	}
	return &protoEnv
}
