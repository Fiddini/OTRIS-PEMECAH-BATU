package monitor
import("bytes";"strings")
type ProtocolFamily string
const(HL7_V2 ProtocolFamily="HL7_V2";ASTM ProtocolFamily="ASTM";BINARY ProtocolFamily="BINARY";UNKNOWN_PF ProtocolFamily="UNKNOWN";DELIMITED ProtocolFamily="DELIMITED")
type Classification struct{Family ProtocolFamily `json:"protocol_family"`;Confidence float64 `json:"confidence"`;ParserIdentity string `json:"parser_identity"`;Reason string `json:"reason"`}
func Classify(d []byte,bc ByteClass,h string,f FramingType)Classification{
 s:=string(d); if strings.Contains(s,"MSH|^~\&"){return Classification{HL7_V2,0.95,"generic_hl7_v2","MSH header"}}
 if strings.Contains(s,"MSH|")&&(f==MLLP||bytes.Contains(d,[]byte("OBX|"))){return Classification{HL7_V2,0.85,"generic_hl7_v2","MSH+OBX+MLLP"}}
 if strings.HasPrefix(s,"H|"){return Classification{ASTM,0.90,"generic_astm","ASTM"}}
 if bc==BINARY{return Classification{BINARY,0.70,"binary_unknown","binary"}}
 if bc==ASCII{return Classification{DELIMITED,0.60,"generic_delimited","delimited"}}
 return Classification{UNKNOWN_PF,0,"unknown","UNKNOWN per constitution"}
}