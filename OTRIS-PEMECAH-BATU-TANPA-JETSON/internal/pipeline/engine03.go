package pipeline
import (
"os"
"github.com/otris-id/otris-pemecah-batu/pkg/canonical"
"github.com/otris-id/otris-pemecah-batu/pkg/monitor"
)
func RunEngine03(path string)(canonical.InspectionReport,error){
 b,err:=os.ReadFile(path); if err!=nil{return canonical.InspectionReport{},err}
 bc,r:=monitor.ClassifyBytes(b); delim,dc:=monitor.DetectDelimiter(b); hdr:=monitor.DetectHeader(b); fp16,fh:=monitor.Fingerprint200(b)
 frCands:=monitor.DetectFraming(b); selFram:=monitor.SelectFraming(frCands)
 classif:=monitor.Classify(b,bc,hdr,selFram)
 var fc []interface{}; for _,c:=range frCands{fc=append(fc,c)}
 return canonical.InspectionReport{
  SourcePath:path, ByteCount:len(b), ByteClass:string(bc), PrintableRatio:r,
  Delimiter:delim, DelimiterConf:dc, HeaderSig:hdr, Fingerprint16:fp16, FullHash:fh,
  FramingCandidates:fc, SelectedFraming:string(selFram), Classification:classif,
  SampleASCII:monitor.SampleASCII(b,200), SampleHex:monitor.SampleHex(b,200),
  Constitution:"UNKNOWN JANGAN DITEBAK - false_data_rate HARUS 0",
 },nil
}