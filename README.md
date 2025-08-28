# demoinfocs-golang - CS2 Demo Parser

A Go library for parsing Counter-Strike 2 and CS:GO demos.

## Setup

### Install Go

1. Download and install Go from [golang.org/dl](https://golang.org/dl/)
2. Verify installation: `go version`

### Install Dependencies

```bash
go get -u github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs
```

## Usage

### Parse Demo to JSON

```bash
go run examples/parse/demo_to_json.go /path/to/demo.dem
```

Output: JSON file with demo data including map, ticks, and events.

### Parse Demo to Text

```bash
go run examples/parse/dem_to_txt.go /path/to/demo.dem
```

Output: Text file with parsed demo events.

## Examples

Both tools are located in `examples/parse/`:
- `demo_to_json.go` - Converts demo to JSON format
- `dem_to_txt.go` - Converts demo to text format

## License

MIT License
