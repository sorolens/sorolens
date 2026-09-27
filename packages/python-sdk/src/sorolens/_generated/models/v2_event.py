from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="V2Event")


@_attrs_define
class V2Event:
    """
    Attributes:
        id (str):
        contract_id (str):
        network (str):
        ledger (int):
        ledger_closed_at (datetime.datetime):
        tx_hash (str):
        type_ (str):
        topic_xdr (list[str]):
        value_xdr (str):
        topic_decoded (list[Any]):
        value_decoded (Any):
        in_successful_call (bool):
    """

    id: str
    contract_id: str
    network: str
    ledger: int
    ledger_closed_at: datetime.datetime
    tx_hash: str
    type_: str
    topic_xdr: list[str]
    value_xdr: str
    topic_decoded: list[Any]
    value_decoded: Any
    in_successful_call: bool
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        contract_id = self.contract_id

        network = self.network

        ledger = self.ledger

        ledger_closed_at = self.ledger_closed_at.isoformat()

        tx_hash = self.tx_hash

        type_ = self.type_

        topic_xdr = self.topic_xdr

        value_xdr = self.value_xdr

        topic_decoded = self.topic_decoded

        value_decoded = self.value_decoded

        in_successful_call = self.in_successful_call

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "contract_id": contract_id,
                "network": network,
                "ledger": ledger,
                "ledger_closed_at": ledger_closed_at,
                "tx_hash": tx_hash,
                "type": type_,
                "topic_xdr": topic_xdr,
                "value_xdr": value_xdr,
                "topic_decoded": topic_decoded,
                "value_decoded": value_decoded,
                "in_successful_call": in_successful_call,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = d.pop("id")

        contract_id = d.pop("contract_id")

        network = d.pop("network")

        ledger = d.pop("ledger")

        ledger_closed_at = datetime.datetime.fromisoformat(d.pop("ledger_closed_at"))

        tx_hash = d.pop("tx_hash")

        type_ = d.pop("type")

        topic_xdr = cast(list[str], d.pop("topic_xdr"))

        value_xdr = d.pop("value_xdr")

        topic_decoded = cast(list[Any], d.pop("topic_decoded"))

        value_decoded = d.pop("value_decoded")

        in_successful_call = d.pop("in_successful_call")

        v2_event = cls(
            id=id,
            contract_id=contract_id,
            network=network,
            ledger=ledger,
            ledger_closed_at=ledger_closed_at,
            tx_hash=tx_hash,
            type_=type_,
            topic_xdr=topic_xdr,
            value_xdr=value_xdr,
            topic_decoded=topic_decoded,
            value_decoded=value_decoded,
            in_successful_call=in_successful_call,
        )

        v2_event.additional_properties = d
        return v2_event

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
