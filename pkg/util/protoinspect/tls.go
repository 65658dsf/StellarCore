package protoinspect

import "encoding/binary"

func DetectTLSClientHello(buf []byte) (bool, string, map[string]string) {
    info := map[string]string{}
    if len(buf) < 9 {
        return false, "", info
    }
    if buf[0] != 0x16 {
        return false, "", info
    }
    if buf[1] != 0x03 {
        return false, "", info
    }
    ver := int(buf[2])
    if ver < 0x01 || ver > 0x04 {
        return false, "", info
    }
    info["TLSMinor"] = string([]byte{buf[2]})
    recLen := int(binary.BigEndian.Uint16(buf[3:5]))
    if 5+recLen > len(buf) {
        recLen = len(buf) - 5
    }
    hs := buf[5 : 5+recLen]
    if len(hs) < 4 {
        return false, "", info
    }
    if hs[0] != 0x01 {
        return false, "", info
    }
    hsLen := int(hs[1])<<16 | int(hs[2])<<8 | int(hs[3])
    if 4+hsLen > len(hs) {
        hsLen = len(hs) - 4
    }
    p := 4
    if p+2 > len(hs) {
        return true, "", info
    }
    p += 2
    if p+32 > len(hs) {
        return true, "", info
    }
    p += 32
    if p+1 > len(hs) {
        return true, "", info
    }
    sidLen := int(hs[p])
    p += 1
    if p+sidLen > len(hs) {
        return true, "", info
    }
    p += sidLen
    if p+2 > len(hs) {
        return true, "", info
    }
    csLen := int(binary.BigEndian.Uint16(hs[p : p+2]))
    p += 2
    if p+csLen > len(hs) {
        return true, "", info
    }
    p += csLen
    if p+1 > len(hs) {
        return true, "", info
    }
    compLen := int(hs[p])
    p += 1
    if p+compLen > len(hs) {
        return true, "", info
    }
    p += compLen
    if p+2 > len(hs) {
        return true, "", info
    }
    extLen := int(binary.BigEndian.Uint16(hs[p : p+2]))
    p += 2
    if p+extLen > len(hs) {
        extLen = len(hs) - p
    }
    end := p + extLen
    for p+4 <= end {
        et := binary.BigEndian.Uint16(hs[p : p+2])
        el := int(binary.BigEndian.Uint16(hs[p+2 : p+4]))
        p += 4
        if p+el > end {
            break
        }
        if et == 0x0000 {
            sni := parseSNI(hs[p : p+el])
            return true, sni, info
        }
        p += el
    }
    return true, "", info
}

func parseSNI(b []byte) string {
    if len(b) < 2 {
        return ""
    }
    listLen := int(binary.BigEndian.Uint16(b[0:2]))
    p := 2
    if p+listLen > len(b) {
        listLen = len(b) - p
    }
    end := p + listLen
    for p+3 <= end {
        nameType := b[p]
        nl := int(binary.BigEndian.Uint16(b[p+1 : p+3]))
        p += 3
        if p+nl > end {
            break
        }
        if nameType == 0 {
            return string(b[p : p+nl])
        }
        p += nl
    }
    return ""
}
