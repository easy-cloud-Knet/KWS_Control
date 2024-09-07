package server

type StatusVMIndex int32

const (
	Status_ALL = iota +1 
	Status_Active 
	Status_Allocated
	Status_Unallocated
	Status_Not_Active
)

type VMStatusNeeded struct{

}