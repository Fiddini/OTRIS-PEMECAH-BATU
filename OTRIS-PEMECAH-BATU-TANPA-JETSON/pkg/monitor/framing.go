package monitor
import "bytes"
type FramingType string
const(MLLP FramingType="MLLP"; NEWLINE FramingType="NEWLINE"; UNKNOWN_FR FramingType="UNKNOWN")
type FramingCandidate struct{Type FramingType `json:"type"`; Confidence float64 `json:"confidence"`; FrameCount int `json:"frame_count"`; Evidence string `json:"evidence"`}
func DetectFraming(b []byte)[]FramingCandidate{
 var c []FramingCandidate; s:=bytes.Count(b,[]byte{0x0B}); e:=bytes.Count(b,[]byte{0x1C,0x0D})
 if s>0&&e>0{c=append(c,FramingCandidate{MLLP,0.95,s,"0x0B..0x1C0D"})}
 nl:=bytes.Count(b,[]byte("\n")); if nl>=2{c=append(c,FramingCandidate{NEWLINE,0.5,nl,"newline"})}
 if len(c)==0{c=append(c,FramingCandidate{UNKNOWN_FR,0.3,1,"unknown"})}
 return c
}
func SelectFraming(cs []FramingCandidate)FramingType{
 if len(cs)==0{return UNKNOWN_FR}; b:=cs[0]; for _,x:=range cs[1:]{if x.Confidence>b.Confidence{b=x}}; if b.Confidence<0.7{return UNKNOWN_FR}; return b.Type
}