from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.create_my_api_key_body_scopes_item import CreateMyApiKeyBodyScopesItem
from ..types import UNSET, Unset

T = TypeVar("T", bound="CreateMyApiKeyBody")


@_attrs_define
class CreateMyApiKeyBody:
    """
    Attributes:
        name (str):
        scopes (list[CreateMyApiKeyBodyScopesItem] | Unset): Defaults to read:contracts when omitted.
    """

    name: str
    scopes: list[CreateMyApiKeyBodyScopesItem] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        scopes: list[str] | Unset = UNSET
        if not isinstance(self.scopes, Unset):
            scopes = []
            for scopes_item_data in self.scopes:
                scopes_item = scopes_item_data.value
                scopes.append(scopes_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "name": name,
            }
        )
        if scopes is not UNSET:
            field_dict["scopes"] = scopes

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        _scopes = d.pop("scopes", UNSET)
        scopes: list[CreateMyApiKeyBodyScopesItem] | Unset = UNSET
        if _scopes is not UNSET:
            scopes = []
            for scopes_item_data in _scopes:
                scopes_item = CreateMyApiKeyBodyScopesItem(scopes_item_data)

                scopes.append(scopes_item)

        create_my_api_key_body = cls(
            name=name,
            scopes=scopes,
        )

        create_my_api_key_body.additional_properties = d
        return create_my_api_key_body

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
