from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

from ..models.create_api_key_body_scopes_item import CreateApiKeyBodyScopesItem
from ..types import UNSET, Unset
from typing import cast
import datetime






T = TypeVar("T", bound="CreateApiKeyBody")



@_attrs_define
class CreateApiKeyBody:
    """ 
        Attributes:
            name (str): Human-friendly key name.
            scopes (list[CreateApiKeyBodyScopesItem]): Scopes granted to the key.
            user_id (str | Unset): Optional user the key acts as. The user must exist. Omit for a scope-only key.
            expires_at (datetime.datetime | Unset): Optional expiry timestamp. Expired keys are rejected by authentication.
     """

    name: str
    scopes: list[CreateApiKeyBodyScopesItem]
    user_id: str | Unset = UNSET
    expires_at: datetime.datetime | Unset = UNSET
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


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "name": name,
            "scopes": scopes,
        })
        if user_id is not UNSET:
            field_dict["user_id"] = user_id
        if expires_at is not UNSET:
            field_dict["expires_at"] = expires_at

        return field_dict



    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        scopes = []
        _scopes = d.pop("scopes")
        for scopes_item_data in (_scopes):
            scopes_item = CreateApiKeyBodyScopesItem(scopes_item_data)



            scopes.append(scopes_item)


        user_id = d.pop("user_id", UNSET)

        _expires_at = d.pop("expires_at", UNSET)
        expires_at: datetime.datetime | Unset
        if isinstance(_expires_at,  Unset):
            expires_at = UNSET
        else:
            expires_at = datetime.datetime.fromisoformat(_expires_at)




        create_api_key_body = cls(
            name=name,
            scopes=scopes,
            user_id=user_id,
            expires_at=expires_at,
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
