# go-ciede2000

Go implementation of the CIE Delta E 2000 Color-Difference algorithm (CIEDE2000).

This is a golang port of https://github.com/gfiumara/CIEDE2000

## Usage

```go
c1 := &color.RGBA{200, 255, 0, 255}
c2 := &color.RGBA{50, 60, 255, 255}
diff := Diff(c1, c2)
```

## Installation

```
$ go get github.com/mattn/go-ciede2000
```

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a. mattn)
