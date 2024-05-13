package main

import (
	"encoding/base32"
	"flag"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// fcid converts fatcat ids to uuids and back, cf. fcid.py

var (
	fromFatcat = flag.String("f", "", "from fatcat id, e.g. container_2ujzwjsay5aohfmwlpyiyhmb7a")
	fromUUID   = flag.String("u", "", "from uuid, e.g. d5139b26-40c7-40e3-9596-5bf08c1d81f8")
)

func main() {
	flag.Parse()
	switch {
	case *fromFatcat != "":
		fmt.Println(fcid2uuid(*fromFatcat))
	case *fromUUID != "":
		fmt.Println(uuid2fcid(*fromUUID))
	}
}

func fcid2uuid(fcid string) string {
	var (
		parts = strings.Split(fcid, "_")
		last  = parts[len(parts)-1]
	)
	if len(last) != 26 {
		return ""
	}
	last = strings.ToUpper(last) + "======"
	b, err := base32.StdEncoding.DecodeString(last)
	if err != nil {
		return ""
	}
	u, err := uuid.FromBytes(b)
	if err != nil {
		return ""
	}
	return u.String()

}
func uuid2fcid(id string) string {
	u, err := uuid.Parse(id)
	if err != nil {
		return ""
	}
	b, err := u.MarshalBinary()
	if err != nil {
		return ""
	}
	return strings.ToLower(base32.StdEncoding.EncodeToString(b))[:26]
}

// Some failed attempts with a few LLMs, latest gemini and ChatGPT fail too.
// -------------------------------------------------------------------------
//
// command-r
// ---------
//
// package main
//
// import (
//         "base64"
//         "fmt"
//         "strings"
//         "unicode/utf8"
//
//         "github.com/google/uuid"
// )
//
// func fcid2uuid(fcid string) string {
//         b := []byte(strings.Split(fcid, "_")[-1])
//         assert(len(b) == 26)
//         b = append(b, bytesToFillUpBase64Encoding(...b)...)
//         rawBytes, _ := base64.StdEncoding.DecodeString(string(b))
//
//         uuid, err := uuid.FromBytes(rawBytes)
//         if err != nil {
//                 panic(err)
//         }
//         return uuid.String()
// }
//
// func uuid2fcid(s string) string {
//         uuid, err := uuid.Parse(s)
//         if err != nil {
//                 panic(err)
//         }
//
//         rawBytes := uuid.Bytes()
//         encoded := base64.StdEncoding.EncodeToString(rawBytes[:8])
//
//         return strings.ToLower(string(append([]byte{byte('f'), byte('c'), byte('i'), byte('d')}, encoded...))) + "_"
// }
//
// func assert(b bool) {
//         if !b {
//                 panic("assert failed")
//         }
// }
//
// func bytesToFillUpBase64Encoding(b ...byte) []byte {
//         if len(b)%4 > 1 {
//                 return []byte{byte('='), byte('='), byte('=')}
//         } else if len(b)%4 == 1 {
//                 return []byte{byte('='), byte('=')}
//         } else if len(b)%4 == 2 {
//                 return []byte{byte('=')}
//         }
//
//         return nil
// }
//
// func test_fcid() {
//         testUUID := "00000000-0000-0000-3333-000000000001"
//         assert(testUUID == fcid2uuid(uuid2fcid(testUUID)))
// }
//
// func main() {
//         fmt.Println("Starting the test...")
//         defer fmt.Println("Test finished")
//
//         test_fcid()
//
//         fmt.Println("All tests passed!")
// }

// phi3
// ----
//
// fcid2uuid converts a Fatcat identifier (base32 encoded string) to a UUID object
// func fcid2uuid(fcid string) string {
// 	b, err := base64.RawLenPrefixDecodeString("_", "======", fcid[1:])
// 	if err != nil {
// 		panic(err)
// 	}
// 	u := uuid.UUID(b[:])
// 	return fmt.Sprintf("%s-%04x%02x%02x-%08x%08x", u.String())
// }
//
// // uuid2fcid converts a UUID object to a Fatcat identifier (base32 encoded string)
// func uuid2fcid(u uuid.UUID) string {
// 	return base64.RawEncodeString("_", "======", u.Bytes())[:26].ToLower()
// }
//
// // testFcid tests the conversion functions by verifying that a UUID can be converted to its Fatcat identifier and back again.
// func testFcid() {
// 	testUuid := uuid.New()
// 	fcid := fcid2uuid(uuid2fcid(testUuid))
// 	if testUuid != uuid2fcid(uuid.FromString(fcid)) {
// 		panic("conversion failed")
// 	}
// }
//
// func main() {
// 	testFcid()
// }

// dolphincoder:15b-starcoder2-q8_0
// --------------------------------
//
// func FCIDToUUID(fcid string) string {
// 	parts := strings.Split(fcid, "_")
// 	b := parts[len(parts)-1]
// 	b = strings.ToUpper(b)
// 	if len(b) != 26 {
// 		panic("Invalid input")
// 	}
// 	rawBytes, err := base32.StdEncoding.DecodeString(b + "======")
// 	if err != nil {
// 		panic(err)
// 	}
// 	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", rawBytes[0:4], rawBytes[4:6], rawBytes[6:8], rawBytes[8:10], rawBytes[10:])
// 	return strings.ToLower(uuid)
// }
//
// func UUIDToFCID(s string) string {
// 	raw, err := base32.StdEncoding.DecodeString(s)
// 	if err != nil {
// 		panic(err)
// 	}
// 	fcid := base32.StdEncoding.EncodeToString(raw[:])
// 	return strings.ToLower(fcid[0:26])
// }
//
// func main() {
// 	testUUID := "00000000-0000-0000-3333-000000000001"
// 	fmt.Println(FCIDToUUID(UUIDToFCID(testUUID))) // prints 00000000-0000-0000-3333-000000000001
// }

// dolphincoder:latest
// -------------------
//
// package main
//
// import (
// 	"encoding/base32"
// 	"strings"
// 	"testing"
//
// 	"github.com/google/uuid"
// )
//
// func FCID2UUID(fcid string) uuid.UUID {
// 	b := strings.ToUpper(strings.Split(fcid, "_")[-1]) + "======"
// 	rawBytes, _ := base32.StdEncoding.DecodeString(b)
// 	return uuid.FromBytesOrNil(rawBytes[:16])
// }
//
// func UUID2FCID(u uuid.UUID) string {
// 	raw := u.Bytes()
// 	encoded := base32.StdEncoding.EncodeToString(raw)
// 	return strings.ToLower(strings.TrimRight(encoded, "="))
// }
//
// func TestFCID(t *testing.T) {
// 	testUUID := uuid.MustParse("00000000-0000-0000-3333-000000000001")
// 	if testUUID != FCID2UUID(UUID2FCID(testUUID)) {
// 		t.Errorf("Test failed: %s", UUID2FCID(testUUID))
// 	}
// }

// CHATGPT 2024-05-13
// ------------------
//
// package main
//
// import (
// 	"encoding/base64"
// 	"fmt"
// 	"strings"
//
// 	"github.com/google/uuid"
// )
//
// func fcid2uuid(fcid string) string {
// 	split := strings.Split(fcid, "_")
// 	b := strings.ToUpper(split[len(split)-1])
// 	b = strings.ReplaceAll(b, "=", "")
// 	b = b + strings.Repeat("=", 8) // Add padding
// 	rawBytes, _ := base64.StdEncoding.DecodeString(b)
// 	return strings.ToLower(uuid.UUID(rawBytes).String())
// }
//
// func uuid2fcid(s string) string {
// 	u, _ := uuid.Parse(s)
// 	raw := u[:]
// 	encoded := base64.StdEncoding.EncodeToString(raw)[:26]
// 	return strings.ToLower(encoded)
// }
//
// func testFcid() {
// 	testUUID := "00000000-0000-0000-3333-000000000001"
// 	if testUUID == fcid2uuid(uuid2fcid(testUUID)) {
// 		fmt.Println("Test passed")
// 	} else {
// 		fmt.Println("Test failed")
// 	}
// }
//
// func main() {
// 	testFcid()
// }
