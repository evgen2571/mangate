"""Validate JSON at the CLI boundary and construct immutable typed records."""

from __future__ import annotations

import types
from collections.abc import Mapping
from dataclasses import MISSING, fields, is_dataclass
from pathlib import Path
from typing import TypeVar, Union, get_args, get_origin, get_type_hints

from .errors import MangateError

T = TypeVar("T")


def decode(model: type[T], data: object) -> T:
    try:
        return _value(model, data)
    except (KeyError, TypeError, ValueError) as error:
        raise MangateError(
            "invalid_result", f"Invalid {model.__name__} result: {error}"
        ) from error


def _value(kind, data):
    origin, args = get_origin(kind), get_args(kind)
    if origin in (Union, types.UnionType):
        if data is None and type(None) in args:
            return None
        return _value(next(arg for arg in args if arg is not type(None)), data)
    if origin is tuple:
        if data is None:
            return ()  # Go nil slices serialize as null.
        if not isinstance(data, list):
            raise ValueError("expected an array")
        return tuple(_value(args[0], item) for item in data)
    if origin is Mapping:
        if not isinstance(data, dict):
            raise ValueError("expected an object")
        return types.MappingProxyType(
            {
                _value(args[0], key): _value(args[1], value)
                for key, value in data.items()
            }
        )
    if is_dataclass(kind):
        if not isinstance(data, dict):
            raise ValueError("expected an object")
        hints = get_type_hints(kind)
        values = {}
        for field in fields(kind):
            words = field.name.split("_")
            key = field.metadata.get(
                "json", words[0] + "".join(word.title() for word in words[1:])
            )
            if key not in data and (
                field.default is not MISSING or field.default_factory is not MISSING
            ):
                continue
            values[field.name] = _value(hints[field.name], data[key])
        return kind(**values)
    if kind is Path:
        if not isinstance(data, str) or not data:
            raise ValueError("expected a nonempty path")
        return Path(data)
    if kind is float and type(data) in (int, float):
        return float(data)
    if type(data) is not kind:
        raise ValueError(f"expected {kind.__name__}, got {type(data).__name__}")
    return data
