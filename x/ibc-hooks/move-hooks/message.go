package move_hooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	movetypes "github.com/initia-labs/initia/x/move/types"
)

// A contract that sends an IBC transfer, may need to listen for the ACK from that packet.
// To allow contracts to listen on the ack of specific packets, we provide Ack callbacks.
//
// The contract, which wants to receive ack callback, have to implement two functions
// - ibc_ack
// - ibc_timeout
//
// public entry fun ibc_ack(
//   callback_id: u64,
//   success:     bool,
// )
//
// public entry fun ibc_timeout(
//   callback_id: u64,
// )
//

const (
	// The memo key is used to parse ics-20 or ics-712 memo fields.
	moveHookMemoKey = "move"

	functionNameAck     = "ibc_ack"
	functionNameTimeout = "ibc_timeout"
)

// AsyncCallback is data wrapper which is required
// when we implement async callback.
type AsyncCallback struct {
	// callback id should be issued form the executor contract
	Id            uint64 `json:"id"`
	ModuleAddress string `json:"module_address"`
	ModuleName    string `json:"module_name"`
}

// HookData defines a wrapper for move execute message
// and async callback.
type HookData struct {
	// Message is a move execute message which will be executed
	// at `OnRecvPacket` of receiver chain.
	Message *movetypes.MsgExecute `json:"message,omitempty"`

	// MessageJSON is a move execute message in JSON format
	// which will be executed at `OnRecvPacket` of receiver chain.
	MessageJSON *movetypes.MsgExecuteJSON `json:"message_json,omitempty"`

	// AsyncCallback is a callback message which will be executed
	// at `OnTimeoutPacket` and `OnAcknowledgementPacket` of
	// sender chain.
	AsyncCallback *AsyncCallback `json:"async_callback,omitempty"`
}

// intermediateCallback is used internally for JSON unmarshaling
type intermediateCallback struct {
	Id            json.RawMessage `json:"id"`
	ModuleAddress string `json:"module_address"`
	ModuleName    string `json:"module_name"`
}

// UnmarshalJSON implements the json unmarshaler interface.
// It handles both string and numeric id formats and validates the module address.
func (a *AsyncCallback) UnmarshalJSON(bz []byte) error {
	var ic intermediateCallback
	if err := json.Unmarshal(bz, &ic); err != nil {
		return fmt.Errorf("failed to unmarshal AsyncCallback: %w", err)
	}

	// Validate required fields
	if ic.ModuleAddress == "" {
		return fmt.Errorf("module_address cannot be empty")
	}
	if ic.ModuleName == "" {
		return fmt.Errorf("module_name cannot be empty")
	}

	// Validate module address format
	if !strings.HasPrefix(ic.ModuleAddress, "0x") {
		return fmt.Errorf("invalid module_address format: must start with '0x'")
	}

	// Parse the ID directly from its JSON representation. Decoding through
	// interface{} would convert JSON numbers to float64 and lose precision
	// for uint64 values greater than 2^53.
	idBytes := bytes.TrimSpace(ic.Id)
	if len(idBytes) == 0 || string(idBytes) == "null" {
		return fmt.Errorf("invalid id type: expected string or number")
	}

	var id uint64
	if idBytes[0] == '"' {
		var idString string
		if err := json.Unmarshal(idBytes, &idString); err != nil {
			return fmt.Errorf("invalid id format: %w", err)
		}
		parsed, err := strconv.ParseUint(idString, 10, 64)
		if err != nil {
			if strings.Contains(err.Error(), "value out of range") || strings.ContainsAny(idString, ".-+") {
				return fmt.Errorf("id value out of range or contains decimals")
			}
			return fmt.Errorf("invalid id format: %w", err)
		}
		id = parsed
	} else if (idBytes[0] >= '0' && idBytes[0] <= '9') || idBytes[0] == '-' {
		parsed, err := strconv.ParseUint(string(idBytes), 10, 64)
		if err != nil {
			return fmt.Errorf("id value out of range or contains decimals")
		}
		id = parsed
	} else {
		return fmt.Errorf("invalid id type: expected string or number")
	}
	a.Id = id

	a.ModuleAddress = ic.ModuleAddress
	a.ModuleName = ic.ModuleName
	return nil
}
