# PlantUML Parallel Composition

A Go tool for composing multiple PlantUML state diagrams in parallel with synchronization events, following CSP (Communicating Sequential Processes) semantics.

## Overview

This tool takes multiple Composable State Diagram files and composes them into a single parallel state diagram with specified synchronization events. The composition follows CSP parallel composition semantics.

## Installation

```bash
go build -o plantuml-parallel-composition
```

## Usage

```bash
./plantuml-parallel-composition [--sync event1;event2;...] <file1.puml> [file2.puml] ...
```

### Options

- `--sync`: Semicolon-separated list of synchronization events for parallel composition

### Examples
```bash
./plantuml-parallel-composition -sync 'insert;showAvailable;showPurchasable;choose;drop' ./examples/user.puml ./examples/vendormachine.puml
```

## Input Format

The tool accepts PlantUML state diagram files in a specific Composable State Diagram format. See the `examples/` directory for sample input files.

## Features

- Parse PlantUML state diagrams
- Compose multiple diagrams in parallel
- Support for synchronization events
- CSP-based parallel composition semantics
- Output in PlantUML format

## Project Structure

- `core/` - Core parsing and composition logic
- `examples/` - Sample PlantUML files
- `docs/` - Documentation including requirements and specifications
- `tools/` - Additional parsing tools

## Limitations

The parallel composition tool has the following limitations:

- **Start edge requirement**: State diagrams without a start edge (`[*] --> state`) cannot be composed in parallel. Each diagram must have exactly one start edge to define the initial state for composition.

- **End edge restriction**: State diagrams containing end edges (`state --> [*]`) are not currently supported for parallel composition. While technically possible, the semantics of parallel composition with terminating processes would be complex to define and implement, so this feature is not yet supported.

These limitations are implementation choices made to keep the parallel composition semantics manageable in the current version.

## Documentation

- [Requirements](docs/REQUIREMENTS.md) - Project requirements (Japanese)
- [Specification](docs/SPEC.md) - Technical specification (Japanese)
- [Syntax](docs/SYNTAX.md) - Syntax documentation
- [Glossary](docs/GLOSSARY.md) - Term definitions

## License

MIT License
