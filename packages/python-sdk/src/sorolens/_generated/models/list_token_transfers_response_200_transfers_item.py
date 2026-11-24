from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.list_token_transfers_response_200_transfers_item_transfer_type import (
    ListTokenTransfersResponse200TransfersItemTransferType,
)
from ..types import UNSET, Unset

T = TypeVar("T", bound="ListTokenTransfersResponse200TransfersItem")


@_attrs_define
class ListTokenTransfersResponse200TransfersItem:
    """
    Attributes:
        id (int):
        contract_id (str):
        network (str):
        tx_hash (str):
        ledger (int):
        ledger_closed_at (datetime.datetime):
        event_id (str):
        transfer_type (ListTokenTransfersResponse200TransfersItemTransferType):
        amount (str):
        from_ (str | Unset):
        to (str | Unset):
    """

    id: int
    contract_id: str
    network: str
    tx_hash: str
    ledger: int
    ledger_closed_at: datetime.datetime
    event_id: str
    transfer_type: ListTokenTransfersResponse200TransfersItemTransferType
    amount: str
    from_: str | Unset = UNSET
    to: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        contract_id = self.contract_id

        network = self.network

        tx_hash = self.tx_hash

        ledger = self.ledger

        ledger_closed_at = self.ledger_closed_at.isoformat()

        event_id = self.event_id

        transfer_type = self.transfer_type.value

        amount = self.amount

        from_ = self.from_

        to = self.to

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "contract_id": contract_id,
                "network": network,
                "tx_hash": tx_hash,
                "ledger": ledger,
                "ledger_closed_at": ledger_closed_at,
                "event_id": event_id,
                "transfer_type": transfer_type,
                "amount": amount,
            }
        )
        if from_ is not UNSET:
            field_dict["from"] = from_
        if to is not UNSET:
            field_dict["to"] = to

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = d.pop("id")

        contract_id = d.pop("contract_id")

        network = d.pop("network")

        tx_hash = d.pop("tx_hash")

        ledger = d.pop("ledger")

        ledger_closed_at = datetime.datetime.fromisoformat(d.pop("ledger_closed_at"))

        event_id = d.pop("event_id")

        transfer_type = ListTokenTransfersResponse200TransfersItemTransferType(
            d.pop("transfer_type")
        )

        amount = d.pop("amount")

        from_ = d.pop("from", UNSET)

        to = d.pop("to", UNSET)

        list_token_transfers_response_200_transfers_item = cls(
            id=id,
            contract_id=contract_id,
            network=network,
            tx_hash=tx_hash,
            ledger=ledger,
            ledger_closed_at=ledger_closed_at,
            event_id=event_id,
            transfer_type=transfer_type,
            amount=amount,
            from_=from_,
            to=to,
        )

        list_token_transfers_response_200_transfers_item.additional_properties = d
        return list_token_transfers_response_200_transfers_item

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
