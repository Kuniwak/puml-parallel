"""The environment function an atom denotes (docs/CLAUSE_DESIGN.md): its name
is the atom's text with each hole marked by the position of the argument it
reads, and its arguments are the distinct terms the holes read, in the order
they first appear. Atoms with the same text
denote the same function, whatever their holes are called.

Holes follow docs/SYNTAX.md: `$name`, with the space after it when the next
character could continue the name, and `${name}`; `$` before anything else is
text, and `\\$`, `\\(` and `\\)` are those characters. The name writes each
hole as `$_N`, N its argument's position, a space after it where the text goes on with a character that
could continue a name, and escapes a `$` of the text.

The walk takes an atom's name from the diagram's JSON; this names a
function a checks module or the command line writes as an atom, so its
arguments are the text of its holes: a name, primed when it is a value
after the step, or the term of a ${…} hole.
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


def has_hole(text: str) -> bool:
    """Whether text holds a hole, so that it is an atom and not a clause of a
    glue's own language."""
    i = 0
    while i < len(text):
        nxt = text[i + 1] if i + 1 < len(text) else ""
        if text[i] == "\\" and nxt in _ESCAPED and nxt:
            i += 2
            continue
        if text[i] == "$" and nxt and (nxt == "{" or _starts_hole_name(nxt)):
            return True
        i += 1
    return False


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
            name = text[i + 2:end].strip()
            if not name:
                raise ValueError(f"the atom {text!r} has a ${{}} hole that reads nothing")
            pieces.append("".join(current))
            current, i = [], end + 1
            holes.append(name)
        elif c == "$" and nxt and _starts_hole_name(nxt):
            j = i + 1
            while j < len(text) and continues_name(text[j]):
                j += 1
            name = text[i + 1:j]
            if j < len(text) and text[j] == "'":
                name += "'"
                j += 1
            if j + 1 < len(text) and text[j] == " " and continues_name(text[j + 1]):
                j += 1
            pieces.append("".join(current))
            current, i = [], j
            holes.append(name)
        else:
            current.append(c)
            i += 1
    pieces.append("".join(current))
    args = tuple(dict.fromkeys(holes))
    return Function(name=_name(pieces, [args.index(h) for h in holes]), args=args)


def _name(pieces: list[str], positions: list[int]) -> str:
    out = [_escaped(pieces[0])]
    for piece, position in zip(pieces[1:], positions):
        out.append(f"{HOLE}{position}")
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
