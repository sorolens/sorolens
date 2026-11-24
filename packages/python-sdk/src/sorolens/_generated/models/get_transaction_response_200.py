from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.event import Event
    from ..models.get_transaction_response_200_trace import (
        GetTransactionResponse200Trace,
    )
    from ..models.invocation import Invocation


T = TypeVar("T", bound="GetTransactionResponse200")


@_attrs_define
class GetTransactionResponse200:
    """
    Attributes:
        tx_hash (str):
        ledger (int):
        events (list[Event]):
        status (str | Unset):
        network (str | Unset):
        invocation (Invocation | Unset):
        trace (GetTransactionResponse200Trace | Unset):
    """

    tx_hash: str
    ledger: int
    events: list[Event]
    status: str | Unset = UNSET
    network: str | Unset = UNSET
    invocation: Invocation | Unset = UNSET
    trace: GetTransactionResponse200Trace | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        tx_hash = self.tx_hash

        ledger = self.ledger

        events = []
        for events_item_data in self.events:
            events_item = events_item_data.to_dict()
            events.append(events_item)

        status = self.status

        network = self.network

        invocation: dict[str, Any] | Unset = UNSET
        if not isinstance(self.invocation, Unset):
            invocation = self.invocation.to_dict()

        trace: dict[str, Any] | Unset = UNSET
        if not isinstance(self.trace, Unset):
            trace = self.trace.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "tx_hash": tx_hash,
                "ledger": ledger,
                "events": events,
            }
        )
        if status is not UNSET:
            field_dict["status"] = status
        if network is not UNSET:
            field_dict["network"] = network
        if invocation is not UNSET:
            field_dict["invocation"] = invocation
        if trace is not UNSET:
            field_dict["trace"] = trace

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.event import Event
        from ..models.get_transaction_response_200_trace import (
            GetTransactionResponse200Trace,
        )
        from ..models.invocation import Invocation

        d = dict(src_dict)
        tx_hash = d.pop("tx_hash")

        ledger = d.pop("ledger")

        events = []
        _events = d.pop("events")
        for events_item_data in _events:
            events_item = Event.from_dict(events_item_data)

            events.append(events_item)

        status = d.pop("status", UNSET)

        network = d.pop("network", UNSET)

        _invocation = d.pop("invocation", UNSET)
        invocation: Invocation | Unset
        if isinstance(_invocation, Unset):
            invocation = UNSET
        else:
            invocation = Invocation.from_dict(_invocation)

        _trace = d.pop("trace", UNSET)
        trace: GetTransactionResponse200Trace | Unset
        if isinstance(_trace, Unset):
            trace = UNSET
        else:
            trace = GetTransactionResponse200Trace.from_dict(_trace)

        get_transaction_response_200 = cls(
            tx_hash=tx_hash,
            ledger=ledger,
            events=events,
            status=status,
            network=network,
            invocation=invocation,
            trace=trace,
        )

        get_transaction_response_200.additional_properties = d
        return get_transaction_response_200

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
