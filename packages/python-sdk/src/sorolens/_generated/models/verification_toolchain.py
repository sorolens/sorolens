from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

T = TypeVar("T", bound="VerificationToolchain")


@_attrs_define
class VerificationToolchain:
    """
    Attributes:
        stellar (str | Unset):
        rustc (str | Unset):
        cargo (str | Unset):
    """

    stellar: str | Unset = UNSET
    rustc: str | Unset = UNSET
    cargo: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        stellar = self.stellar

        rustc = self.rustc

        cargo = self.cargo

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if stellar is not UNSET:
            field_dict["stellar"] = stellar
        if rustc is not UNSET:
            field_dict["rustc"] = rustc
        if cargo is not UNSET:
            field_dict["cargo"] = cargo

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        stellar = d.pop("stellar", UNSET)

        rustc = d.pop("rustc", UNSET)

        cargo = d.pop("cargo", UNSET)

        verification_toolchain = cls(
            stellar=stellar,
            rustc=rustc,
            cargo=cargo,
        )

        verification_toolchain.additional_properties = d
        return verification_toolchain

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
