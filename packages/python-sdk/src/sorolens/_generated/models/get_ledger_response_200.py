from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.get_ledger_response_200_top_contracts_item import (
        GetLedgerResponse200TopContractsItem,
    )


T = TypeVar("T", bound="GetLedgerResponse200")


@_attrs_define
class GetLedgerResponse200:
    """
    Attributes:
        sequence (int):
        tx_count (int):
        invocation_count (int):
        failed_count (int):
        event_count (int):
        top_contracts (list[GetLedgerResponse200TopContractsItem]):
        closed_at (datetime.datetime | Unset):
    """

    sequence: int
    tx_count: int
    invocation_count: int
    failed_count: int
    event_count: int
    top_contracts: list[GetLedgerResponse200TopContractsItem]
    closed_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        sequence = self.sequence

        tx_count = self.tx_count

        invocation_count = self.invocation_count

        failed_count = self.failed_count

        event_count = self.event_count

        top_contracts = []
        for top_contracts_item_data in self.top_contracts:
            top_contracts_item = top_contracts_item_data.to_dict()
            top_contracts.append(top_contracts_item)

        closed_at: str | Unset = UNSET
        if not isinstance(self.closed_at, Unset):
            closed_at = self.closed_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "sequence": sequence,
                "tx_count": tx_count,
                "invocation_count": invocation_count,
                "failed_count": failed_count,
                "event_count": event_count,
                "top_contracts": top_contracts,
            }
        )
        if closed_at is not UNSET:
            field_dict["closed_at"] = closed_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.get_ledger_response_200_top_contracts_item import (
            GetLedgerResponse200TopContractsItem,
        )

        d = dict(src_dict)
        sequence = d.pop("sequence")

        tx_count = d.pop("tx_count")

        invocation_count = d.pop("invocation_count")

        failed_count = d.pop("failed_count")

        event_count = d.pop("event_count")

        top_contracts = []
        _top_contracts = d.pop("top_contracts")
        for top_contracts_item_data in _top_contracts:
            top_contracts_item = GetLedgerResponse200TopContractsItem.from_dict(
                top_contracts_item_data
            )

            top_contracts.append(top_contracts_item)

        _closed_at = d.pop("closed_at", UNSET)
        closed_at: datetime.datetime | Unset
        if isinstance(_closed_at, Unset):
            closed_at = UNSET
        else:
            closed_at = datetime.datetime.fromisoformat(_closed_at)

        get_ledger_response_200 = cls(
            sequence=sequence,
            tx_count=tx_count,
            invocation_count=invocation_count,
            failed_count=failed_count,
            event_count=event_count,
            top_contracts=top_contracts,
            closed_at=closed_at,
        )

        get_ledger_response_200.additional_properties = d
        return get_ledger_response_200

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
