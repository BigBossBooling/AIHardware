package gen

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestVMConfigMessage(t *testing.T) {
	original := &VMConfig{
		VmId:      "test-vm-001",
		MemoryMb:  4096,
		VcpuCount: 4,
	}

	// Test field setting and getting
	if original.GetVmId() != "test-vm-001" {
		t.Errorf("GetVmId() = %v, want %v", original.GetVmId(), "test-vm-001")
	}
	if original.GetMemoryMb() != 4096 {
		t.Errorf("GetMemoryMb() = %v, want %v", original.GetMemoryMb(), 4096)
	}
	if original.GetVcpuCount() != 4 {
		t.Errorf("GetVcpuCount() = %v, want %v", original.GetVcpuCount(), 4)
	}

	// Test Marshal and Unmarshal roundtrip
	marshaledData, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("proto.Marshal failed: %v", err)
	}

	unmarshaled := &VMConfig{}
	err = proto.Unmarshal(marshaledData, unmarshaled)
	if err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	// Verify unmarshaled data
	if unmarshaled.GetVmId() != original.GetVmId() {
		t.Errorf("Unmarshaled VmId = %v, want %v", unmarshaled.GetVmId(), original.GetVmId())
	}
	if unmarshaled.GetMemoryMb() != original.GetMemoryMb() {
		t.Errorf("Unmarshaled MemoryMb = %v, want %v", unmarshaled.GetMemoryMb(), original.GetMemoryMb())
	}
	if unmarshaled.GetVcpuCount() != original.GetVcpuCount() {
		t.Errorf("Unmarshaled VcpuCount = %v, want %v", unmarshaled.GetVcpuCount(), original.GetVcpuCount())
	}

	// Test with a different message type (e.g., StartVMRequest)
	startReqOriginal := &StartVMRequest{
		Config: original,
	}

	if startReqOriginal.GetConfig().GetVmId() != "test-vm-001" {
		t.Errorf("startReqOriginal.GetConfig().GetVmId() = %v, want %v", startReqOriginal.GetConfig().GetVmId(), "test-vm-001")
	}

	marshaledStartReq, err := proto.Marshal(startReqOriginal)
	if err != nil {
		t.Fatalf("proto.Marshal(startReqOriginal) failed: %v", err)
	}

	unmarshaledStartReq := &StartVMRequest{}
	err = proto.Unmarshal(marshaledStartReq, unmarshaledStartReq)
	if err != nil {
		t.Fatalf("proto.Unmarshal(unmarshaledStartReq) failed: %v", err)
	}

	if unmarshaledStartReq.GetConfig().GetVmId() != startReqOriginal.GetConfig().GetVmId() {
		t.Errorf("Unmarshaled StartVMRequest VmId = %v, want %v", unmarshaledStartReq.GetConfig().GetVmId(), startReqOriginal.GetConfig().GetVmId())
	}
	if unmarshaledStartReq.GetConfig().GetMemoryMb() != startReqOriginal.GetConfig().GetMemoryMb() {
		t.Errorf("Unmarshaled StartVMRequest MemoryMb = %v, want %v", unmarshaledStartReq.GetConfig().GetMemoryMb(), startReqOriginal.GetConfig().GetMemoryMb())
	}

}

func TestGetHypervisorInfoRequestResponse(t *testing.T) {
	req := &GetHypervisorInfoRequest{} // Empty request

	// Marshal/Unmarshal Request
	marshaledReq, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("proto.Marshal(GetHypervisorInfoRequest) failed: %v", err)
	}
	unmarshaledReq := &GetHypervisorInfoRequest{}
	err = proto.Unmarshal(marshaledReq, unmarshaledReq)
	if err != nil {
		t.Fatalf("proto.Unmarshal(GetHypervisorInfoRequest) failed: %v", err)
	}

	respOriginal := &GetHypervisorInfoResponse{
		HypervisorType: "KVM",
		Version:        "1.2.3",
	}

	if respOriginal.GetHypervisorType() != "KVM" {
		t.Errorf("respOriginal.GetHypervisorType() = %v, want %v", respOriginal.GetHypervisorType(), "KVM")
	}

	// Marshal/Unmarshal Response
	marshaledResp, err := proto.Marshal(respOriginal)
	if err != nil {
		t.Fatalf("proto.Marshal(GetHypervisorInfoResponse) failed: %v", err)
	}
	unmarshaledResp := &GetHypervisorInfoResponse{}
	err = proto.Unmarshal(marshaledResp, unmarshaledResp)
	if err != nil {
		t.Fatalf("proto.Unmarshal(GetHypervisorInfoResponse) failed: %v", err)
	}
	if unmarshaledResp.GetHypervisorType() != respOriginal.GetHypervisorType() {
		t.Errorf("Unmarshaled HypervisorType = %v, want %v", unmarshaledResp.GetHypervisorType(), respOriginal.GetHypervisorType())
	}
	if unmarshaledResp.GetVersion() != respOriginal.GetVersion() {
		t.Errorf("Unmarshaled Version = %v, want %v", unmarshaledResp.GetVersion(), respOriginal.GetVersion())
	}
}
