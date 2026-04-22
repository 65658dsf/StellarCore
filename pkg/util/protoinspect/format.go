package protoinspect

import "strconv"

func uint16ToString(v uint16) string {
	return strconv.FormatUint(uint64(v), 10)
}

func uint32ToString(v uint32) string {
	return strconv.FormatUint(uint64(v), 10)
}

func strconvItoa(v int) string {
	return strconv.Itoa(v)
}
