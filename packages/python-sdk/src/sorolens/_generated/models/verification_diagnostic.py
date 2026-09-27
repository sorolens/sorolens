from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.verification_diagnostic_severity import VerificationDiagnosticSeverity
from ..types import UNSET, Unset

T = TypeVar("T", bound="VerificationDiagnostic")


@_attrs_define
class VerificationDiagnostic:
    """
    Attributes:
        code (str):
        severity (VerificationDiagnosticSeverity):
        message (str):
        hint (str | Unset): Actionable suggestion for resolving the finding.
    """

    code: str
    severity: VerificationDiagnosticSeverity
    message: str
    hint: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        code = self.code

        severity = self.severity.value

        message = self.message

        hint = self.hint

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "code": code,
                "severity": severity,
                "message": message,
            }
        )
        if hint is not UNSET:
            field_dict["hint"] = hint

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        code = d.pop("code")

        severity = VerificationDiagnosticSeverity(d.pop("severity"))

        message = d.pop("message")

        hint = d.pop("hint", UNSET)

        verification_diagnostic = cls(
            code=code,
            severity=severity,
            message=message,
            hint=hint,
        )

        verification_diagnostic.additional_properties = d
        return verification_diagnostic

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
