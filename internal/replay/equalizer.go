package replay

import "math"

type shelfFilter struct {
	b0, b1, b2, a1, a2 float64
	z1, z2             float64
}

func (f *shelfFilter) sample(input float64) float64 {
	output := f.b0*input + f.z1
	f.z1 = f.b1*input - f.a1*output + f.z2
	f.z2 = f.b2*input - f.a2*output
	return output
}

// Digital shelving uses the native control's 2 dB steps. It preserves the
// audible bass/treble range, while not claiming an analog LMC1992 circuit model.
func (f *shelfFilter) configure(rate int, frequency, gainDB float64, high bool) {
	a := math.Pow(10, gainDB/40)
	omega := 2 * math.Pi * frequency / float64(rate)
	c, s := math.Cos(omega), math.Sin(omega)
	alpha := s / 2 * math.Sqrt(2)
	beta := 2 * math.Sqrt(a) * alpha
	var b0, b1, b2, a0, a1, a2 float64
	if high {
		b0 = a * ((a + 1) + (a-1)*c + beta)
		b1 = -2 * a * ((a - 1) + (a+1)*c)
		b2 = a * ((a + 1) + (a-1)*c - beta)
		a0 = (a + 1) - (a-1)*c + beta
		a1 = 2 * ((a - 1) - (a+1)*c)
		a2 = (a + 1) - (a-1)*c - beta
	} else {
		b0 = a * ((a + 1) - (a-1)*c + beta)
		b1 = 2 * a * ((a - 1) - (a+1)*c)
		b2 = a * ((a + 1) - (a-1)*c - beta)
		a0 = (a + 1) + (a-1)*c + beta
		a1 = -2 * ((a - 1) + (a+1)*c)
		a2 = (a + 1) + (a-1)*c - beta
	}
	f.b0, f.b1, f.b2, f.a1, f.a2 = b0/a0, b1/a0, b2/a0, a1/a0, a2/a0
}

type equalizer struct {
	bass, treble int
	initialized  bool
	low, high    shelfFilter
}

func (e *equalizer) process(sample int, rate, bass, treble int) int {
	if !e.initialized || e.bass != bass || e.treble != treble {
		e.low.configure(rate, 100, float64(bass-6)*2, false)
		e.high.configure(rate, 10000, float64(treble-6)*2, true)
		e.bass, e.treble, e.initialized = bass, treble, true
	}
	if bass == 6 && treble == 6 {
		return sample
	}
	value := e.high.sample(e.low.sample(float64(sample)))
	return int(math.Round(value))
}
