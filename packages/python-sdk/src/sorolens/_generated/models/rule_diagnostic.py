from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

T = TypeVar("T", bound="RuleDiagnostic")


@_attrs_define
class RuleDiagnostic:
    """A rule validation problem, positioned for the dashboard editor.

    Attributes:
        message (str | Unset):
        hint (str | Unset):
        line (int | Unset):
        column (int | Unset):
    """

    message: str | Unset = UNSET
    hint: str | Unset = UNSET
    line: int | Unset = UNSET
    column: int | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        message = self.message

        hint = self.hint

        line = self.line

        column = self.column

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if message is not UNSET:
            field_dict["message"] = message
        if hint is not UNSET:
            field_dict["hint"] = hint
        if line is not UNSET:
            field_dict["line"] = line
        if column is not UNSET:
            field_dict["column"] = column

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        message = d.pop("message", UNSET)

        hint = d.pop("hint", UNSET)

        line = d.pop("line", UNSET)

        column = d.pop("column", UNSET)

        rule_diagnostic = cls(
            message=message,
            hint=hint,
            line=line,
            column=column,
        )

        rule_diagnostic.additional_properties = d
        return rule_diagnostic

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
