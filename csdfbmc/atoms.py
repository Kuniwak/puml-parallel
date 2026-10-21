"""The environment function an atom denotes (docs/CLAUSE_DESIGN.md): its name
is the atom's text without its holes, and its arguments are the holes, each
distinct one once, in the order they first appear. Atoms with the same text
denote the same function, whatever their holes are called.

Holes follow docs/SYNTAX.md: `$name`, with the space after it when the next
character could continue the name, and `${name}`; `$` before anything else is
text, and `\\$`, `\\(` and `\\)` are those characters. The name writes each
hole as `$_`, a space after it where the text goes on with a character that
could continue a name, and escapes a `$` of the text.

Until the walk reads atoms in their wire form (#76, step 6), csdfbmc reads
them from the clause's text here, and only what a guard reads: a hole of a
name, never primed.
"""
from __future__ import annotations

import unicodedata
from dataclasses import dataclass

_ESCAPED = "$()"
HOLE = "$_"


@dataclass(frozen=True)
class Function:
    name: str
    args: tuple[str, ...]


def continues_name(c: str) -> bool:
    """Whether c may appear in an identifier (csdf/ident)."""
    return c == "_" or unicodedata.category(c)[0] in "LN"


def _starts_hole_name(c: str) -> bool:
    return continues_name(c) and unicodedata.category(c) != "Nd"


def function_of(text: str) -> Function:
    """The function the atom text denotes."""
    pieces, holes, current, i = [], [], [], 0
    while i < len(text):
        c = text[i]
        nxt = text[i + 1] if i + 1 < len(text) else ""
        if c == "\\" and nxt in _ESCAPED and nxt:
            current.append(nxt)
            i += 2
        elif c == "$" and nxt == "{":
            end = text.find("}", i)
            if end < 0:
                raise ValueError(f"the atom {text!r} opens ${{ with no closing brace")
            name = text[i + 2:end]
            if name.endswith("'"):
                raise ValueError(f"the atom {text!r} reads {name}, a value after the step, which a guard does not read")
            if not name or not all(continues_name(x) for x in name):
                raise ValueError(f"the atom {text!r} reads ${{{name}}}; csdfbmc reads only a name in a hole until it reads atoms (#76)")
            pieces.append("".join(current))
            current, i = [], end + 1
            holes.append(name)
        elif c == "$" and nxt and _starts_hole_name(nxt):
            j = i + 1
            while j < len(text) and continues_name(text[j]):
                j += 1
            name = text[i + 1:j]
            if j < len(text) and text[j] == "'":
                raise ValueError(f"the atom {text!r} reads {name}', a value after the step, which a guard does not read")
            if j + 1 < len(text) and text[j] == " " and continues_name(text[j + 1]):
                j += 1
            pieces.append("".join(current))
            current, i = [], j
            holes.append(name)
        else:
            current.append(c)
            i += 1
    pieces.append("".join(current))
    return Function(name=_name(pieces), args=tuple(dict.fromkeys(holes)))


def _name(pieces: list[str]) -> str:
    out = [_escaped(pieces[0])]
    for piece in pieces[1:]:
        out.append(HOLE)
        if piece and continues_name(piece[0]):
            out.append(" ")
        out.append(_escaped(piece))
    return "".join(out)


def _escaped(piece: str) -> str:
    """piece with every $ that would start a hole escaped."""
    out = []
    for k, c in enumerate(piece):
        nxt = piece[k + 1] if k + 1 < len(piece) else ""
        out.append("\\$" if c == "$" and (nxt == "{" or (nxt and _starts_hole_name(nxt))) else c)
    return "".join(out)
