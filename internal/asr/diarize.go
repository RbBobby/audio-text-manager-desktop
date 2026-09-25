package asr

import (
	"encoding/binary"
	"io"
	"math"
	"os"
)

const (
	wavRate       = 16000
	maxSpeakers   = 2
	minSegSamples = wavRate / 10
	featDim       = 10
)

// AssignSpeakers labels segments as at most two speakers (teacher / student).
func AssignSpeakers(wavPath string, segs []Segment) {
	if len(segs) == 0 {
		return
	}
	pcm, err := readPCM16Mono16k(wavPath)
	if err != nil || len(pcm) < minSegSamples {
		pauseSpeakers(segs)
		return
	}
	vecs := make([][]float64, len(segs))
	ok := 0
	for i, s := range segs {
		v := embedSegment(pcm, s.StartMS, s.EndMS)
		if v != nil {
			ok++
		}
		vecs[i] = v
	}
	if ok < 2 {
		for i := range segs {
			segs[i].Speaker = 1
		}
		return
	}
	fillMissing(vecs)
	ids := clusterEmbeddings(vecs)
	for i := range segs {
		if ids[i] < 1 {
			ids[i] = 1
		}
		segs[i].Speaker = ids[i]
	}
	smoothSpeakers(segs)
}

func pauseSpeakers(segs []Segment) {
	sp := 1
	for i := range segs {
		if i > 0 && segs[i].StartMS-segs[i-1].EndMS >= 800 {
			sp++
			if sp > 2 {
				sp = 1
			}
		}
		segs[i].Speaker = sp
	}
}

func embedSegment(pcm []int16, startMS, endMS int) []float64 {
	a := startMS * wavRate / 1000
	b := endMS * wavRate / 1000
	if a < 0 {
		a = 0
	}
	if b > len(pcm) {
		b = len(pcm)
	}
	if b-a < minSegSamples {
		return nil
	}
	chunk := pcm[a:b]
	const win = 512
	hop := 256
	acc := make([]float64, featDim)
	n := 0
	for i := 0; i+win <= len(chunk); i += hop {
		frame := chunk[i : i+win]
		energy := 0.0
		zcr := 0.0
		for j, s := range frame {
			x := float64(s) / 32768.0
			energy += x * x
			if j > 0 && (frame[j-1] >= 0) != (s >= 0) {
				zcr++
			}
		}
		energy = math.Log1p(energy / float64(win))
		zcr /= float64(win)
		bands := bandEnergy(frame)
		centroid := 0.0
		denom := 0.0
		for k, e := range bands {
			centroid += float64(k+1) * e
			denom += e
		}
		if denom > 0 {
			centroid /= denom
		}
		acc[0] += energy
		acc[1] += zcr
		acc[2] += centroid
		for k := 0; k < 7 && k < len(bands); k++ {
			acc[3+k] += bands[k]
		}
		n++
	}
	if n == 0 {
		return nil
	}
	for i := range acc {
		acc[i] /= float64(n)
	}
	norm := 0.0
	for _, v := range acc {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm < 1e-9 {
		return nil
	}
	for i := range acc {
		acc[i] /= norm
	}
	return acc
}

func bandEnergy(frame []int16) []float64 {
	freqs := []float64{250, 500, 900, 1400, 2200, 3200, 5000}
	out := make([]float64, len(freqs))
	for i, f := range freqs {
		out[i] = goertzel(frame, f, wavRate)
	}
	return out
}

func goertzel(frame []int16, freq, rate float64) float64 {
	n := len(frame)
	k := int(0.5 + float64(n)*freq/rate)
	w := 2 * math.Pi * float64(k) / float64(n)
	coeff := 2 * math.Cos(w)
	s0, s1, s2 := 0.0, 0.0, 0.0
	for _, x := range frame {
		s0 = float64(x)/32768.0 + coeff*s1 - s2
		s2, s1 = s1, s0
	}
	return math.Log1p(s1*s1 + s2*s2 - coeff*s1*s2)
}

func fillMissing(vecs [][]float64) {
	var last []float64
	for i, v := range vecs {
		if v != nil {
			last = v
			continue
		}
		if last != nil {
			vecs[i] = last
		}
	}
	var next []float64
	for i := len(vecs) - 1; i >= 0; i-- {
		if vecs[i] != nil {
			next = vecs[i]
			continue
		}
		if next != nil {
			vecs[i] = next
		}
	}
}

func clusterEmbeddings(vecs [][]float64) []int {
	id := make([]int, len(vecs))
	next := 1
	for i, v := range vecs {
		if v == nil {
			id[i] = 1
			continue
		}
		id[i] = next
		next++
	}
	for uniqueCount(id) > maxSpeakers {
		if !mergeClosest(vecs, id) {
			break
		}
	}
	return remapIDs(id)
}

func smoothSpeakers(segs []Segment) {
	if len(segs) < 2 {
		return
	}
	for i := 1; i < len(segs)-1; i++ {
		if segs[i].Speaker == segs[i-1].Speaker || segs[i].Speaker == segs[i+1].Speaker {
			continue
		}
		dur := segs[i].EndMS - segs[i].StartMS
		if dur < 250 || isRepeatText(segs[i-1].Text, segs[i].Text) {
			segs[i].Speaker = segs[i-1].Speaker
		}
	}
	for i := 1; i < len(segs); i++ {
		if segs[i].Speaker != segs[i-1].Speaker && isRepeatText(segs[i-1].Text, segs[i].Text) {
			segs[i].Speaker = segs[i-1].Speaker
		}
	}
}

func mergeClosest(vecs [][]float64, id []int) bool {
	d, a, b := closestClusters(vecs, id)
	if a == 0 || b == 0 {
		return false
	}
	_ = d
	mergeCluster(id, a, b)
	return true
}

func closestClusters(vecs [][]float64, id []int) (dist float64, a, b int) {
	seen := map[int][]int{}
	for i, c := range id {
		seen[c] = append(seen[c], i)
	}
	dist = 1e9
	clusters := make([]int, 0, len(seen))
	for c := range seen {
		clusters = append(clusters, c)
	}
	for i := 0; i < len(clusters); i++ {
		for j := i + 1; j < len(clusters); j++ {
			d := clusterDist(vecs, seen[clusters[i]], seen[clusters[j]])
			if d < dist {
				dist = d
				a, b = clusters[i], clusters[j]
			}
		}
	}
	return dist, a, b
}

func mergeCluster(id []int, from, to int) {
	if from > to {
		from, to = to, from
	}
	for i := range id {
		if id[i] == to {
			id[i] = from
		}
	}
}

func uniqueCount(id []int) int {
	u := map[int]struct{}{}
	for _, v := range id {
		u[v] = struct{}{}
	}
	return len(u)
}

func clusterDist(vecs [][]float64, a, b []int) float64 {
	best := 1e9
	for _, i := range a {
		if vecs[i] == nil {
			continue
		}
		for _, j := range b {
			if vecs[j] == nil {
				continue
			}
			d := cosineDist(vecs[i], vecs[j])
			if d < best {
				best = d
			}
		}
	}
	return best
}

func cosineDist(a, b []float64) float64 {
	dot := 0.0
	for i := range a {
		dot += a[i] * b[i]
	}
	if dot > 1 {
		dot = 1
	}
	if dot < -1 {
		dot = -1
	}
	return 1 - dot
}

func remapIDs(id []int) []int {
	m := map[int]int{}
	next := 1
	out := make([]int, len(id))
	for i, v := range id {
		if _, ok := m[v]; !ok {
			m[v] = next
			next++
		}
		out[i] = m[v]
	}
	return out
}

func readPCM16Mono16k(path string) ([]int16, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	hdr := make([]byte, 12)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return nil, err
	}
	if string(hdr[0:4]) != "RIFF" || string(hdr[8:12]) != "WAVE" {
		return nil, io.ErrUnexpectedEOF
	}
	var data []byte
	buf := make([]byte, 8)
	for {
		if _, err := io.ReadFull(f, buf); err != nil {
			break
		}
		id := string(buf[0:4])
		sz := binary.LittleEndian.Uint32(buf[4:8])
		body := make([]byte, sz)
		if _, err := io.ReadFull(f, body); err != nil {
			return nil, err
		}
		if sz%2 == 1 {
			_, _ = f.Read(make([]byte, 1))
		}
		if id == "data" {
			data = body
			break
		}
	}
	if len(data) < 2 {
		return nil, io.ErrUnexpectedEOF
	}
	n := len(data) / 2
	out := make([]int16, n)
	for i := 0; i < n; i++ {
		out[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	return out, nil
}
