package structure

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestControlContext_AddInstance_ExecutesInsertQueries(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := &ControlContext{DB: db}
	vm := &VMInfo{
		UUID:         UUID("a7e8fa1b-3b4d-4e6f-80d4-6e3019c2f105"),
		IP_VM:        "10.0.0.31",
		GuacPassword: "guac-pass",
		Memory:       4096,
		Cpu:          4,
		Disk:         60,
	}
	coreIdx := 2

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO inst_info \(uuid, inst_ip, guac_pass, inst_mem, inst_vcpu, inst_disk\) VALUES \(\?, \?, \?, \?, \?, \?\)`).
		WithArgs(string(vm.UUID), vm.IP_VM, vm.GuacPassword, vm.Memory, vm.Cpu, vm.Disk).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO inst_loc \(uuid, core\) VALUES \(\?, \?\)`).
		WithArgs(string(vm.UUID), coreIdx).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := ctx.AddInstance(vm, coreIdx); err != nil {
		t.Fatalf("AddInstance returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestControlContext_UpdateInstance_ExecutesUpdateQuery(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := &ControlContext{DB: db}
	vm := &VMInfo{
		UUID:   UUID("d5870f4c-d12b-467d-8e1f-68ad0f89d227"),
		IP_VM:  "10.0.0.41",
		Memory: 8192,
		Cpu:    8,
		Disk:   120,
	}

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE inst_info SET inst_ip = \?, inst_mem = \?, inst_vcpu = \?, inst_disk = \? WHERE uuid = \?`).
		WithArgs(vm.IP_VM, vm.Memory, vm.Cpu, vm.Disk, string(vm.UUID)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := ctx.UpdateInstance(vm); err != nil {
		t.Fatalf("UpdateInstance returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}
