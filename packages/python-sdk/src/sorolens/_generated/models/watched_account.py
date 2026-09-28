from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

T = TypeVar("T", bound="WatchedAccount")


@_attrs_define
class WatchedAccount:
    """
    Attributes:
        account_id (str): Stellar account ID (56 chars, starts with G).
        discovered_count (int): Contracts deployed by this account that the indexer has tracked.
        created_at (datetime.datetime):
        added_by (str | Unset): Actor that registered the account; empty when seeded internally.
    """

    account_id: str
    discovered_count: int
    created_at: datetime.datetime
    added_by: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        account_id = self.account_id

        discovered_count = self.discovered_count

        created_at = self.created_at.isoformat()

        added_by = self.added_by

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "account_id": account_id,
                "discovered_count": discovered_count,
                "created_at": created_at,
            }
        )
        if added_by is not UNSET:
            field_dict["added_by"] = added_by

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        account_id = d.pop("account_id")

        discovered_count = d.pop("discovered_count")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        added_by = d.pop("added_by", UNSET)

        watched_account = cls(
            account_id=account_id,
            discovered_count=discovered_count,
            created_at=created_at,
            added_by=added_by,
        )

        watched_account.additional_properties = d
        return watched_account

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
