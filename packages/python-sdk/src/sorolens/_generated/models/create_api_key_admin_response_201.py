from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

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
        user_id (None | str | Unset): User this key acts as. Null for scope-only keys.
        rotated_at (datetime.datetime | None | Unset): When the key's secret was last rotated in place.
        expires_at (datetime.datetime | None | Unset): When the key expires, or null if it does not expire.
        monthly_quota (int | None | Unset): Monthly request quota, or null if unmetered.
        allowed_cidrs (list[str] | Unset): Source networks allowed to use this key, as normalized CIDR ranges
            (a bare IP is stored as `/32` or `/128`). Empty means any address.
            Requests from outside the list are rejected with `403` and error
            code `API_KEY_RESTRICTED`.
        allowed_origins (list[str] | Unset): Browser Origins allowed to use this key. Empty means any origin. A
            request whose `Origin` header is not listed is rejected with `403`
            and error code `API_KEY_RESTRICTED`.
    """

    id: str
    name: str
    key_prefix: str
    scopes: list[str]
    created_at: datetime.datetime
    last_used_at: datetime.datetime | None
    revoked_at: datetime.datetime | None
    key: str
    user_id: None | str | Unset = UNSET
    rotated_at: datetime.datetime | None | Unset = UNSET
    expires_at: datetime.datetime | None | Unset = UNSET
    monthly_quota: int | None | Unset = UNSET
    allowed_cidrs: list[str] | Unset = UNSET
    allowed_origins: list[str] | Unset = UNSET
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

        user_id: None | str | Unset
        if isinstance(self.user_id, Unset):
            user_id = UNSET
        else:
            user_id = self.user_id

        rotated_at: None | str | Unset
        if isinstance(self.rotated_at, Unset):
            rotated_at = UNSET
        elif isinstance(self.rotated_at, datetime.datetime):
            rotated_at = self.rotated_at.isoformat()
        else:
            rotated_at = self.rotated_at

        expires_at: None | str | Unset
        if isinstance(self.expires_at, Unset):
            expires_at = UNSET
        elif isinstance(self.expires_at, datetime.datetime):
            expires_at = self.expires_at.isoformat()
        else:
            expires_at = self.expires_at

        monthly_quota: int | None | Unset
        if isinstance(self.monthly_quota, Unset):
            monthly_quota = UNSET
        else:
            monthly_quota = self.monthly_quota

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
        if user_id is not UNSET:
            field_dict["user_id"] = user_id
        if rotated_at is not UNSET:
            field_dict["rotated_at"] = rotated_at
        if expires_at is not UNSET:
            field_dict["expires_at"] = expires_at
        if monthly_quota is not UNSET:
            field_dict["monthly_quota"] = monthly_quota
        if allowed_cidrs is not UNSET:
            field_dict["allowed_cidrs"] = allowed_cidrs
        if allowed_origins is not UNSET:
            field_dict["allowed_origins"] = allowed_origins

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

        def _parse_user_id(data: object) -> None | str | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(None | str | Unset, data)

        user_id = _parse_user_id(d.pop("user_id", UNSET))

        def _parse_rotated_at(data: object) -> datetime.datetime | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                rotated_at_type_0 = datetime.datetime.fromisoformat(data)

                return rotated_at_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None | Unset, data)

        rotated_at = _parse_rotated_at(d.pop("rotated_at", UNSET))

        def _parse_expires_at(data: object) -> datetime.datetime | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                expires_at_type_0 = datetime.datetime.fromisoformat(data)

                return expires_at_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None | Unset, data)

        expires_at = _parse_expires_at(d.pop("expires_at", UNSET))

        def _parse_monthly_quota(data: object) -> int | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(int | None | Unset, data)

        monthly_quota = _parse_monthly_quota(d.pop("monthly_quota", UNSET))

        allowed_cidrs = cast(list[str], d.pop("allowed_cidrs", UNSET))

        allowed_origins = cast(list[str], d.pop("allowed_origins", UNSET))

        create_api_key_admin_response_201 = cls(
            id=id,
            name=name,
            key_prefix=key_prefix,
            scopes=scopes,
            created_at=created_at,
            last_used_at=last_used_at,
            revoked_at=revoked_at,
            key=key,
            user_id=user_id,
            rotated_at=rotated_at,
            expires_at=expires_at,
            monthly_quota=monthly_quota,
            allowed_cidrs=allowed_cidrs,
            allowed_origins=allowed_origins,
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
