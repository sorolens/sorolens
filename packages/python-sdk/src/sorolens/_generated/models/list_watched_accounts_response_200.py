from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.watched_account import WatchedAccount


T = TypeVar("T", bound="ListWatchedAccountsResponse200")


@_attrs_define
class ListWatchedAccountsResponse200:
    """
    Attributes:
        watched_accounts (list[WatchedAccount]):
    """

    watched_accounts: list[WatchedAccount]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        watched_accounts = []
        for watched_accounts_item_data in self.watched_accounts:
            watched_accounts_item = watched_accounts_item_data.to_dict()
            watched_accounts.append(watched_accounts_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "watched_accounts": watched_accounts,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.watched_account import WatchedAccount

        d = dict(src_dict)
        watched_accounts = []
        _watched_accounts = d.pop("watched_accounts")
        for watched_accounts_item_data in _watched_accounts:
            watched_accounts_item = WatchedAccount.from_dict(watched_accounts_item_data)

            watched_accounts.append(watched_accounts_item)

        list_watched_accounts_response_200 = cls(
            watched_accounts=watched_accounts,
        )

        list_watched_accounts_response_200.additional_properties = d
        return list_watched_accounts_response_200

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
