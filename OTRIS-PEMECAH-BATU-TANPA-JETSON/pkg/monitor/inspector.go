package monitor
import (
 "bytes"
 "crypto/sha256"
 "encoding/hex"
 "strings"
)
type ByteClass string
const (ASCII ByteClass="ASCII"; BINARY ByteClass="BINARY"; MIXED ByteClass="MIXED")
func ClassifyBytes(b []byte) (ByteClass,float64) {
 if len(b)==0{return BINARY,0}
 p:=0; for _,c:=range b{if(c>=32&&c<=126)||c==9||c==10||c==13||c==11||c==28{p++}}
 r:=float64(p)/float64(len(b))
 if r>0.90{return ASCII,r}; if r<0.30{return BINARY,r}; return MIXED,r
}
func DetectDelimiter(b []byte)(string,float64){
 pipe:=bytes.Count(b,[]byte("|")); caret:=bytes.Count(b,[]byte("^")); comma:=bytes.Count(b,[]byte(","))
 best:=""; bc:=0; if pipe>=3{best="|";bc=pipe}; if caret>=3&&caret>bc{best="^";bc=caret}; if comma>=3&&comma>bc{best=",";bc=comma}
 if best==""{return "",0}; conf:=float64(bc)/float64(len(b))*50; if conf>1{conf=1}; return best,conf
}
func DetectHeader(b []byte)string{
 s:=string(b); if strings.Contains(s,"MSH|^~&"){return "MSH|^~&"}; if strings.Contains(s,"MSH|"){return "MSH|"}
 if bytes.HasPrefix(b,[]byte{0x0B}){return "MLLP_START"}; if bytes.Contains(b,[]byte{0x1C,0x0D}){return "MLLP_END"}
 if strings.HasPrefix(s,"H|")||strings.Contains(s,"P|1|"){return "ASTM_H_P"}; return ""
}
func Fingerprint200(b []byte)(string,string){
 n:=200; if len(b)<n{n=len(b)}; if n==0{return "empty","sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}
 h:=sha256.Sum256(b[:n]); hf:=hex.EncodeToString(h[:]); return hf[:16],"sha256:"+hf
}
func SampleASCII(b []byte,max int)string{if len(b)>max{b=b[:max}}; out:=make([]byte,len(b)); for i,c:=range b{if c>=32&&c<=126{out[i]=c}else{out[i]=46}}; return string(out)}
func SampleHex(b []byte,max int)string{if len(b)>max{b=b[:max}}; return hex.EncodeToString(b)}