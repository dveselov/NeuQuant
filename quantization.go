package quantization

import (
	"math"

	"image"
	"image/color"

	"github.com/a-h/round"
)

const (
	RADIUS_DEC      = 30                   // factor of 1/30 each cycle
	ALPHA_BIASSHIFT = 10                   // alpha starts at 1
	INIT_ALPHA      = 1 << ALPHA_BIASSHIFT // biased by 10 bits
	GAMMA           = 1024.0
	BETA            = 1.0 / GAMMA
	BETAGAMMA       = BETA * GAMMA
	NCYCLES         = 600
)

type Neuron struct {
	r float64
	g float64
	b float64
	a float64
}

type Color struct {
	r uint32
	g uint32
	b uint32
	a uint32
}

type NeuQuant struct {
	Network    []*Neuron
	Colormap   []*Color
	Netindex   []int
	Bias       []float64
	Freq       []float64
	Samplefrac int
	Netsize    int
}

// Initializes the neuronal network and trains it with the supplied data
func (n *NeuQuant) Init(img image.Image) {
	n.Network = nil
	n.Colormap = nil
	n.Bias = nil
	n.Freq = nil
	freq := float64(n.Netsize)
	for i := 0; i < n.Netsize; i++ {
		tmp := float64(i)
		a := float64(255)
		n.Network = append(n.Network, &Neuron{r: tmp, g: tmp, b: tmp, a: a})
		n.Colormap = append(n.Colormap, &Color{r: 0, g: 0, b: 0, a: 0})
		n.Bias = append(n.Bias, 0.0)
		n.Freq = append(n.Freq, freq)
	}
	for i := 0; i < 1<<16; i++ {
		n.Netindex = append(n.Netindex, 0)
	}
	n.Learn(img)
	n.buildColormap()
	n.buildIndex()
}

// Search for biased BGR values
// finds closest neuron (min dist) and updates freq
// finds best neuron (min dist-bias) and returns position
// for frequently chosen neurons, freq[i] is high and bias[i] is negative
// bias[i] = gamma*((1/self.netsize)-freq[i])
func (n *NeuQuant) contest(b, g, r, a float64) int {
	bestd := math.MaxFloat64
	bestbiasd := bestd
	bestpos := -1
	bestbiaspos := bestpos

	for i := 0; i < n.Netsize; i++ {
		bestbiasd_biased := bestbiasd + n.Bias[i]
		var dist float64
		neuron := n.Network[i]
		dist = math.Abs((neuron.b - b))
		dist += math.Abs((neuron.r - r))
		if dist < bestd || dist < bestbiasd_biased {
			dist += math.Abs((neuron.g - g))
			dist += math.Abs((neuron.a - a))
			if dist < bestd {
				bestd = dist
				bestpos = i
			}
			biasdist := dist - n.Bias[i]
			if biasdist < bestbiasd {
				bestbiasd = biasdist
				bestbiaspos = i
			}
		}
		n.Freq[i] -= BETA * n.Freq[i]
		n.Bias[i] += BETAGAMMA * n.Freq[i]
	}
	n.Freq[bestpos] += BETA
	n.Bias[bestpos] -= BETAGAMMA
	return bestbiaspos
}

// Move neuron i towards biased (a,b,g,r) by factor alpha
func (n *NeuQuant) alterSingle(alpha float64, i int, quad Neuron) {
	neuron := n.Network[i]
	neuron.b -= alpha * (neuron.b - quad.b)
	neuron.g -= alpha * (neuron.g - quad.g)
	neuron.r -= alpha * (neuron.r - quad.r)
	neuron.a -= alpha * (neuron.a - quad.a)
}

// Move neuron adjacent neurons towards biased (a,b,g,r) by factor alpha
func (n *NeuQuant) alterNeigh(alpha float64, rad int, i int, quad Neuron) {
	lo := int(math.Max(float64(i-rad), 0.0))
	hi := int(math.Min(float64(i+rad), float64(n.Netsize)))
	j := i + 1
	k := i - 1
	q := 0
	for {
		if (j < hi) || (k > lo) {
			rad_sq := float64(rad) * float64(rad)
			alpha_ := (alpha * (rad_sq - float64(q)*float64(q))) / rad_sq
			q += 1
			if j < hi {
				neuron := n.Network[j]
				neuron.b -= alpha_ * (neuron.b - quad.b)
				neuron.g -= alpha_ * (neuron.g - quad.g)
				neuron.r -= alpha_ * (neuron.r - quad.r)
				neuron.a -= alpha_ * (neuron.a - quad.a)
				j += 1
			}
			if k > lo {
				neuron := n.Network[k]
				neuron.b -= alpha_ * (neuron.b - quad.b)
				neuron.g -= alpha_ * (neuron.g - quad.g)
				neuron.r -= alpha_ * (neuron.r - quad.r)
				neuron.a -= alpha_ * (neuron.a - quad.a)
				k -= 1
			}
		} else {
			break
		}
	}
}

// Main learning loop
// Note: the number of learning cycles is crucial and the parameters are not
// optimized for net sizes < 26 or > 256. 1064 colors seems to work fine
func (n *NeuQuant) Learn(img image.Image) {
	bounds := img.Bounds()
	initrad := n.Netsize / 8
	radiusbiasshift := uint(6)
	radiusbias := 1 << radiusbiasshift
	init_bias_radius := initrad * radiusbias
	bias_radius := init_bias_radius
	alphadec := float64(30<<8 + ((n.Samplefrac - 1) / 3))

	delta := (bounds.Max.X * bounds.Max.Y) / n.Samplefrac / NCYCLES
	if delta <= 0 {
		delta = 1
	}

	alpha := float64(INIT_ALPHA)
	rad := bias_radius >> radiusbiasshift
	if rad <= 1 {
		rad = 0
	}

	for x := 0; x < bounds.Max.X; x++ {
		for y := 0; y < bounds.Max.Y; y++ {
			r, g, b, a := img.At(x, y).RGBA()
			r_f64, g_f64, b_f64, a_f64 := float64(r), float64(g), float64(b), float64(a)

			j := n.contest(b_f64, g_f64, r_f64, a_f64)

			alpha_ := (1.0 * alpha) / float64(INIT_ALPHA)
			n.alterSingle(alpha_, j, Neuron{b: b_f64, g: g_f64, r: r_f64, a: a_f64})
			if rad > 0 {
				n.alterNeigh(alpha_, rad, j, Neuron{b: b_f64, g: g_f64, r: r_f64, a: a_f64})
			}
			if (x > 0 && y > 0) && (x*y)%delta == 0 {
				alpha -= alpha / alphadec
				bias_radius -= bias_radius / RADIUS_DEC
				rad = bias_radius >> radiusbiasshift
				if rad <= 1 {
					rad = 0
				}
			}
		}
	}
}

func clamp(x float64) uint32 {
	r := uint32(round.AwayFromZero(x, 0))
	if r < 0 {
		return uint32(0)
	} else if r > uint32(math.MaxInt32) {
		return uint32(math.MaxInt32)
	} else {
		return r
	}
}

// initializes the color map
func (n *NeuQuant) buildColormap() {
	for i := 0; i < n.Netsize; i++ {
		n.Colormap[i].b = clamp(n.Network[i].b)
		n.Colormap[i].g = clamp(n.Network[i].g)
		n.Colormap[i].r = clamp(n.Network[i].r)
		n.Colormap[i].a = clamp(n.Network[i].a)
	}
}

// Insertion sort of network and building of netindex[0..1<<16]
func (n *NeuQuant) buildIndex() {
	previousCol := 0
	startpos := 0
	for i := 0; i < n.Netsize; i++ {
		p := n.Colormap[i]
		var q *Color
		smallpos := i
		smallval := p.g //index on g
		// find smallest in i..netsize-1
		for j := i; j < n.Netsize; j++ {
			q = n.Colormap[j]
			if (q.g) < smallval {
				smallpos = j
				smallval = q.g
			}
		}
		q = n.Colormap[smallpos]
		// swap p (i) and q (smallpos) entries
		if i != smallpos {
			var j *Color
			j = q
			q = p
			p = j
			n.Colormap[i] = p
			n.Colormap[smallpos] = q
		}
		// smallval entry is now in position i
		if int(smallval) != previousCol {
			n.Netindex[previousCol] = (startpos + i)
			for j := previousCol; j < int(smallval); j++ {
				n.Netindex[j] = i
			}
			previousCol = int(smallval)
			startpos = i
		}
	}
	max_netpos := n.Netsize - 1
	n.Netindex[previousCol] = (startpos + max_netpos)
	for j := previousCol; j < 1<<16; j++ {
		n.Netindex[j] = max_netpos
	}
}

// Search for best matching color
func (n *NeuQuant) indexSearch(b, g, r, a uint32) int {
	bestd := uint32(math.MaxUint32)
	best := 0
	i := n.Netindex[int(g)]
	var j int
	if i > 0 {
		j = i - 1
	} else {
		j = 0
	}
	for {
		if i < n.Netsize || j > 0 {
			if i < n.Netsize {
				p := n.Colormap[i]
				e := p.g - g
				dist := e * e
				if dist >= bestd {
					break
				} else {
					e = p.b - b
					dist += e * e
					if dist < bestd {
						e = p.r - r
						dist += e * e
						if dist < bestd {
							e = p.a - a
							dist += e * e
							if dist < bestd {
								bestd = dist
								best = i
							}
						}
					}
					i += 1
				}
			}
			if j > 0 {
				p := n.Colormap[j]
				e := p.g - g
				dist := e * e
				if dist >= bestd {
					break
				} else {
					e = p.b - b
					dist += e * e
					if dist < bestd {
						e = p.r - r
						dist += e * e
						if dist < bestd {
							e = p.a - a
							dist += e * e
							if dist < bestd {
								bestd = dist
								best = j
							}
						}
					}
					j -= 1
				}
			}
		} else {
			break
		}
	}
	return best
}

func (n *NeuQuant) GetPalette() []color.Color {
	palette := make([]color.Color, 0, n.Netsize)
	for i := 0; i < n.Netsize; i++ {
		c := n.Colormap[i]
		palette = append(palette, color.RGBA{
			R: uint8(c.r >> 8),
			G: uint8(c.g >> 8),
			B: uint8(c.b >> 8),
			A: uint8(c.a >> 8),
		})
	}
	return palette
}

func NewNeuquant(samplefrac, colors int, img image.Image) NeuQuant {
	q := NeuQuant{
		Network:    make([]*Neuron, 0, colors),
		Colormap:   make([]*Color, 0, colors),
		Netindex:   make([]int, 0, 1<<16),
		Bias:       make([]float64, 0, colors),
		Freq:       make([]float64, 0, colors),
		Samplefrac: samplefrac,
		Netsize:    colors,
	}
	q.Init(img)
	return q
}
