from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

T = TypeVar("T", bound="VerificationExpectedInput")


@_attrs_define
class VerificationExpectedInput:
    """
    Attributes:
        stellar_version (str | Unset): Stellar CLI version the contract was originally built with. A
            mismatch is reported as a diagnostic rather than a hard failure.
        rustc_version (str | Unset): rustc version the contract was originally built with.
    """

    stellar_version: str | Unset = UNSET
    rustc_version: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        stellar_version = self.stellar_version

        rustc_version = self.rustc_version

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if stellar_version is not UNSET:
            field_dict["stellar_version"] = stellar_version
        if rustc_version is not UNSET:
            field_dict["rustc_version"] = rustc_version

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        stellar_version = d.pop("stellar_version", UNSET)

        rustc_version = d.pop("rustc_version", UNSET)

        verification_expected_input = cls(
            stellar_version=stellar_version,
            rustc_version=rustc_version,
        )

        verification_expected_input.additional_properties = d
        return verification_expected_input

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
