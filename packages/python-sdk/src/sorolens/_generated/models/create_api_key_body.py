from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.create_api_key_body_scopes_item import CreateApiKeyBodyScopesItem
from ..types import UNSET, Unset

T = TypeVar("T", bound="CreateApiKeyBody")


@_attrs_define
class CreateApiKeyBody:
    """
    Attributes:
        name (str): Human-friendly key name.
        scopes (list[CreateApiKeyBodyScopesItem]): Scopes granted to the key.
        user_id (str | Unset): Optional user the key acts as. The user must exist. Omit for a scope-only key.
        expires_at (datetime.datetime | Unset): Optional expiry timestamp. Expired keys are rejected by authentication.
        allowed_cidrs (list[str] | Unset): Source networks allowed to use this key. Each entry is a
            CIDR range (`203.0.113.0/24`, `2001:db8::/32`) or a bare IP
            address, which is stored as a single-host range. Invalid
            entries are rejected with `422`. Omit or send an empty list
            for an unrestricted key.
        allowed_origins (list[str] | Unset): Browser Origins allowed to use this key, for example
            `https://app.example.com`. Scheme and host only: paths,
            queries and wildcards are rejected with `422`. Omit or send
            an empty list to allow any origin.
    """

    name: str
    scopes: list[CreateApiKeyBodyScopesItem]
    user_id: str | Unset = UNSET
    expires_at: datetime.datetime | Unset = UNSET
    allowed_cidrs: list[str] | Unset = UNSET
    allowed_origins: list[str] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        scopes = []
        for scopes_item_data in self.scopes:
            scopes_item = scopes_item_data.value
            scopes.append(scopes_item)

        user_id = self.user_id

        expires_at: str | Unset = UNSET
        if not isinstance(self.expires_at, Unset):
            expires_at = self.expires_at.isoformat()

        allowed_cidrs: list[str] | Unset = UNSET
        if not isinstance(self.allowed_cidrs, Unset):
            allowed_cidrs = self.allowed_cidrs

        allowed_origins: list[str] | Unset = UNSET
        if not isinstance(self.allowed_origins, Unset):
            allowed_origins = self.allowed_origins

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "name": name,
                "scopes": scopes,
            }
        )
        if user_id is not UNSET:
            field_dict["user_id"] = user_id
        if expires_at is not UNSET:
            field_dict["expires_at"] = expires_at
        if allowed_cidrs is not UNSET:
            field_dict["allowed_cidrs"] = allowed_cidrs
        if allowed_origins is not UNSET:
            field_dict["allowed_origins"] = allowed_origins

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        scopes = []
        _scopes = d.pop("scopes")
        for scopes_item_data in _scopes:
            scopes_item = CreateApiKeyBodyScopesItem(scopes_item_data)

            scopes.append(scopes_item)

        user_id = d.pop("user_id", UNSET)

        _expires_at = d.pop("expires_at", UNSET)
        expires_at: datetime.datetime | Unset
        if isinstance(_expires_at, Unset):
            expires_at = UNSET
        else:
            expires_at = datetime.datetime.fromisoformat(_expires_at)

        allowed_cidrs = cast(list[str], d.pop("allowed_cidrs", UNSET))

        allowed_origins = cast(list[str], d.pop("allowed_origins", UNSET))

        create_api_key_body = cls(
            name=name,
            scopes=scopes,
            user_id=user_id,
            expires_at=expires_at,
            allowed_cidrs=allowed_cidrs,
            allowed_origins=allowed_origins,
        )

        create_api_key_body.additional_properties = d
        return create_api_key_body

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
