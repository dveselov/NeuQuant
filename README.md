# NeuQuant

Neural network based color quantizer.

Almost one-to-one port of [this rust implementation](http://www.piston.rs/image/color_quant/struct.NeuQuant.html).

## Install

```bash
$ go get github.com/dveselov/NeuQuant
```

## Usage

```go
// Open & read image file
file, _ := os.Open("image.png")
reader := bufio.NewReader(file)
img, _ := png.Decode(reader)
bounds := img.Bounds()

// Quantize image to 256 colors and create color palette
q := NewNeuquant(10, 256, img)
palette := q.GetPalette()

// Create new paketted image
dist := image.NewPaletted(bounds, palette)

// Iterate over all pixels in source image
// and set appropriate color in output image
for x := 0; x < bounds.Max.X; x++ {
    for y := 0; y < bounds.Max.Y; y++ {
        r, g, b, a := img.At(x, y).RGBA()
        index := q.indexSearch(b, g, r, a)
        c := q.Colormap[index]
        dist.Set(x, y, color.RGBA{
            R: uint8(c.r >> 8),
            G: uint8(c.g >> 8),
            B: uint8(c.b >> 8),
            A: uint8(c.a >> 8),
        })
    }
}

out, err := os.Create("output.png")
if err != nil {
    panic(err)
}
writer := bufio.NewWriter(out)
err = png.Encode(writer, dist)
if err != nil {
    panic(err)
}
writer.Flush()
```

## Examples

<table>
    <tr>
        <td>Source image</td>
        <td>Result image</td>
        <td>Source filesize</td>
        <td>Result filesize</td>
    </tr>
    <tr>
        <td>
            <a href="https://i.imgur.com/0MDDNv0.jpg">
                <img src="https://i.imgur.com/0MDDNv0.jpg" width="64px" height="64px">
            </a>
        </td>
        <td>
            <a href="https://i.imgur.com/qE9JE6b.jpg">
                <img src="https://i.imgur.com/qE9JE6b.jpg" width="64px" height="64px">
            </a>
        </td>
        <td>50 KB</td>
        <td>11 KB</td>
    </tr>
    <tr>
        <td>
            <a href="https://i.imgur.com/J5PbW57.png">
                <img src="https://i.imgur.com/J5PbW57.png" width="64px" height="64px">
            </a>
        </td>
        <td>
            <a href="https://i.imgur.com/4DmsgzG.png">
                <img src="https://i.imgur.com/4DmsgzG.png" width="64px" height="64px">
            </a>
        </td>
        <td>148 KB</td>
        <td>56 KB</td>
    </tr>
</table>
