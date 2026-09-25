from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="CreateApiKeyAdminResponse201")


@_attrs_define
class CreateApiKeyAdminResponse201:
    """
    Attributes:
        id (str):
        name (str):
        key_prefix (str):
        scopes (list[str]):
        created_at (datetime.datetime):
        last_used_at (datetime.datetime | None):
        revoked_at (datetime.datetime | None):
        key (str):
    """

    id: str
    name: str
    key_prefix: str
    scopes: list[str]
    created_at: datetime.datetime
    last_used_at: datetime.datetime | None
    revoked_at: datetime.datetime | None
    key: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        name = self.name

        key_prefix = self.key_prefix

        scopes = self.scopes

        created_at = self.created_at.isoformat()

        last_used_at: None | str
        if isinstance(self.last_used_at, datetime.datetime):
            last_used_at = self.last_used_at.isoformat()
        else:
            last_used_at = self.last_used_at

        revoked_at: None | str
        if isinstance(self.revoked_at, datetime.datetime):
            revoked_at = self.revoked_at.isoformat()
        else:
            revoked_at = self.revoked_at

        key = self.key

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "name": name,
                "key_prefix": key_prefix,
                "scopes": scopes,
                "created_at": created_at,
                "last_used_at": last_used_at,
                "revoked_at": revoked_at,
                "key": key,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = d.pop("id")

        name = d.pop("name")

        key_prefix = d.pop("key_prefix")

        scopes = cast(list[str], d.pop("scopes"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        def _parse_last_used_at(data: object) -> datetime.datetime | None:
            if data is None:
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                last_used_at_type_0 = datetime.datetime.fromisoformat(data)

                return last_used_at_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None, data)

        last_used_at = _parse_last_used_at(d.pop("last_used_at"))

        def _parse_revoked_at(data: object) -> datetime.datetime | None:
            if data is None:
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                revoked_at_type_0 = datetime.datetime.fromisoformat(data)

                return revoked_at_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None, data)

        revoked_at = _parse_revoked_at(d.pop("revoked_at"))

        key = d.pop("key")

        create_api_key_admin_response_201 = cls(
            id=id,
            name=name,
            key_prefix=key_prefix,
            scopes=scopes,
            created_at=created_at,
            last_used_at=last_used_at,
            revoked_at=revoked_at,
            key=key,
        )

        create_api_key_admin_response_201.additional_properties = d
        return create_api_key_admin_response_201

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
