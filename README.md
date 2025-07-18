# PlantUML Interface Parallel

A Go tool for composing multiple PlantUML state diagrams in parallel with synchronization events, following CSP (Communicating Sequential Processes) semantics.

## Overview

This tool takes multiple Composable State Diagram files and composes them into a single parallel state diagram with specified synchronization events. The composition follows CSP interface parallel semantics.

## Installation

Download binaries from [Releases](https://github.com/Kuniwak/puml-parallel/releases).

## Usage

```console
$ puml-parallel [--sync event1;event2;...] <file1.puml> [file2.puml] ...
```

### Options

- `--sync`: Semicolon-separated list of synchronization events for interface parallel

### Examples
```console
$ puml-parallel -sync 'insert;showAvailable;showPurchasable;choose;drop' ./examples/user.puml ./examples/vendormachine.puml
```

## Input Format
The tool accepts PlantUML state diagram files in a specific Composable State Diagram format. See the [SYNTAX.md](./docs/SYNTAX.md) and `examples/` directory for sample input files.

## Project Structure

- `core/` - Core parsing and composition logic
- `examples/` - Sample PlantUML files
- `docs/` - Documentation including requirements and specifications
- `tools/` - Additional parsing tools

## Limitations

The interface parallel tool has the following limitations:

- **Start edge requirement**: State diagrams without a start edge (`[*] --> state`) cannot be composed in parallel. Each diagram must have exactly one start edge to define the initial state for composition.

- **End edge restriction**: State diagrams containing end edges (`state --> [*]`) are not currently supported for interface parallel. While technically possible, the semantics of interface parallel with terminating processes would be complex to define and implement, so this feature is not yet supported.

These limitations are implementation choices made to keep the interface parallel semantics manageable in the current version.

## Documentation

- [Requirements](docs/REQUIREMENTS.md) - Project requirements (Japanese)
- [Specification](docs/SPEC.md) - Technical specification (Japanese)
- [Syntax](docs/SYNTAX.md) - Syntax documentation
- [Glossary](docs/GLOSSARY.md) - Term definitions

## License

MIT License
