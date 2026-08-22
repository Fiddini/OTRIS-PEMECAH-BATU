package canonical
type InspectionReport struct{
 SourcePath string `json:"source_path"`
 ByteCount int `json:"byte_count"`
 ByteClass string `json:"byte_class"`
 PrintableRatio float64 `json:"printable_ratio"`
 Delimiter string `json:"delimiter"`
 DelimiterConf float64 `json:"delimiter_confidence"`
 HeaderSig string `json:"header_signature"`
 Fingerprint16 string `json:"fingerprint_16"`
 FullHash string `json:"full_sha256"`
 FramingCandidates []interface{} `json:"framing_candidates"`
 SelectedFraming string `json:"selected_framing"`
 Classification interface{} `json:"classification"`
 SampleASCII string `json:"sample_ascii"`
 SampleHex string `json:"sample_hex"`
 Constitution string `json:"constitution"`
}