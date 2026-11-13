package registry

// Wijoyo Intelligence, tanpa protokol, meski belum kenal tetap jalan
// Kalau volume tidak wajar atau target di luar izin atau coba kirim mentah, tandai tidak lazim
func CheckPemakaianTidakLazim(volume int, targetDiizinkan bool, cobaKirimMentah bool) bool {
	if volume > 20 || targetDiizinkan == false || cobaKirimMentah == true {
		return true
	}
	return false
}
