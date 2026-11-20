from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

T = TypeVar("T", bound="APIKeyAllowlistUpdate")


@_attrs_define
class APIKeyAllowlistUpdate:
    """Partial update of an API key's source allowlists (issue #793). Omit a field to leave it unchanged; send an empty
    array or null to clear it.

        Attributes:
            allowed_cidrs (list[str] | Unset): CIDR ranges the key may be used from. Empty clears the restriction.
            allowed_origins (list[str] | Unset): Browser origins the key may be used from. Empty clears the restriction.
    """

    allowed_cidrs: list[str] | Unset = UNSET
    allowed_origins: list[str] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        allowed_cidrs: list[str] | Unset = UNSET
        if not isinstance(self.allowed_cidrs, Unset):
            allowed_cidrs = self.allowed_cidrs

        allowed_origins: list[str] | Unset = UNSET
        if not isinstance(self.allowed_origins, Unset):
            allowed_origins = self.allowed_origins

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({})
        if allowed_cidrs is not UNSET:
            field_dict["allowed_cidrs"] = allowed_cidrs
        if allowed_origins is not UNSET:
            field_dict["allowed_origins"] = allowed_origins

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        allowed_cidrs = cast(list[str], d.pop("allowed_cidrs", UNSET))

        allowed_origins = cast(list[str], d.pop("allowed_origins", UNSET))

        api_key_allowlist_update = cls(
            allowed_cidrs=allowed_cidrs,
            allowed_origins=allowed_origins,
        )

        api_key_allowlist_update.additional_properties = d
        return api_key_allowlist_update

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
