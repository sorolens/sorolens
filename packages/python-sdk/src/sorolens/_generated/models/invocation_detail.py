from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.failure_category import FailureCategory
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.event import Event
    from ..models.invocation_args_decoded import InvocationArgsDecoded


T = TypeVar("T", bound="InvocationDetail")


@_attrs_define
class InvocationDetail:
    """
    Attributes:
        tx_hash (str):
        contract_id (str):
        network (str):
        ledger (int):
        ledger_closed_at (datetime.datetime):
        status (str):
        function_name (str):
        args_decoded (InvocationArgsDecoded):
        result_decoded (Any):
        result_xdr (str):
        resource_fee_charged (int):
        cpu_insn (int):
        mem_byte (int):
        ledger_read_byte (int):
        ledger_write_byte (int):
        application_order (int):
        events (list[Event]):
        failure_category (FailureCategory | Unset): Why a failed invocation failed, derived from the Soroban host error
            and the InvokeHostFunction result code (see docs/failure-categories.md). Set only on FAILED invocations; omitted
            for every other status and for invocations indexed before classification existed. budget_exceeded: the
            invocation ran out of declared resources (host error type Budget, or result RESOURCE_LIMIT_EXCEEDED /
            INSUFFICIENT_REFUNDABLE_FEE). contract_error: the contract returned its own error code (type Contract).
            auth_failure: authorization failed (type Auth). storage: a ledger entry was missing or archived (type Storage,
            or result ENTRY_ARCHIVED). wasm_trap: the contract's Wasm trapped (type WasmVm). other: anything else, including
            unknown or undecodable errors; failure_code keeps the raw value.
        failure_code (str | Unset): Raw error behind failure_category, kept for every failed invocation: the host error
            as "Error(<Type>, <Code>)" (for example "Error(Contract, #4)" or "Error(Budget, ExceededLimit)"), else the
            operation result code ("INVOKE_HOST_FUNCTION_TRAPPED") or transaction result code ("txSOROBAN_INVALID"), or
            "UNKNOWN" when nothing could be decoded. Unknown enum values are kept as numbers, for example "Error(#12, #3)".
            Omitted with failure_category. Example: Error(Contract, #4).
    """

    tx_hash: str
    contract_id: str
    network: str
    ledger: int
    ledger_closed_at: datetime.datetime
    status: str
    function_name: str
    args_decoded: InvocationArgsDecoded
    result_decoded: Any
    result_xdr: str
    resource_fee_charged: int
    cpu_insn: int
    mem_byte: int
    ledger_read_byte: int
    ledger_write_byte: int
    application_order: int
    events: list[Event]
    failure_category: FailureCategory | Unset = UNSET
    failure_code: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        tx_hash = self.tx_hash

        contract_id = self.contract_id

        network = self.network

        ledger = self.ledger

        ledger_closed_at = self.ledger_closed_at.isoformat()

        status = self.status

        function_name = self.function_name

        args_decoded = self.args_decoded.to_dict()

        result_decoded = self.result_decoded

        result_xdr = self.result_xdr

        resource_fee_charged = self.resource_fee_charged

        cpu_insn = self.cpu_insn

        mem_byte = self.mem_byte

        ledger_read_byte = self.ledger_read_byte

        ledger_write_byte = self.ledger_write_byte

        application_order = self.application_order

        events = []
        for events_item_data in self.events:
            events_item = events_item_data.to_dict()
            events.append(events_item)

        failure_category: str | Unset = UNSET
        if not isinstance(self.failure_category, Unset):
            failure_category = self.failure_category.value

        failure_code = self.failure_code

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "tx_hash": tx_hash,
                "contract_id": contract_id,
                "network": network,
                "ledger": ledger,
                "ledger_closed_at": ledger_closed_at,
                "status": status,
                "function_name": function_name,
                "args_decoded": args_decoded,
                "result_decoded": result_decoded,
                "result_xdr": result_xdr,
                "resource_fee_charged": resource_fee_charged,
                "cpu_insn": cpu_insn,
                "mem_byte": mem_byte,
                "ledger_read_byte": ledger_read_byte,
                "ledger_write_byte": ledger_write_byte,
                "application_order": application_order,
                "events": events,
            }
        )
        if failure_category is not UNSET:
            field_dict["failure_category"] = failure_category
        if failure_code is not UNSET:
            field_dict["failure_code"] = failure_code

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.event import Event
        from ..models.invocation_args_decoded import InvocationArgsDecoded

        d = dict(src_dict)
        tx_hash = d.pop("tx_hash")

        contract_id = d.pop("contract_id")

        network = d.pop("network")

        ledger = d.pop("ledger")

        ledger_closed_at = datetime.datetime.fromisoformat(d.pop("ledger_closed_at"))

        status = d.pop("status")

        function_name = d.pop("function_name")

        args_decoded = InvocationArgsDecoded.from_dict(d.pop("args_decoded"))

        result_decoded = d.pop("result_decoded")

        result_xdr = d.pop("result_xdr")

        resource_fee_charged = d.pop("resource_fee_charged")

        cpu_insn = d.pop("cpu_insn")

        mem_byte = d.pop("mem_byte")

        ledger_read_byte = d.pop("ledger_read_byte")

        ledger_write_byte = d.pop("ledger_write_byte")

        application_order = d.pop("application_order")

        events = []
        _events = d.pop("events")
        for events_item_data in _events:
            events_item = Event.from_dict(events_item_data)

            events.append(events_item)

        _failure_category = d.pop("failure_category", UNSET)
        failure_category: FailureCategory | Unset
        if isinstance(_failure_category, Unset):
            failure_category = UNSET
        else:
            failure_category = FailureCategory(_failure_category)

        failure_code = d.pop("failure_code", UNSET)

        invocation_detail = cls(
            tx_hash=tx_hash,
            contract_id=contract_id,
            network=network,
            ledger=ledger,
            ledger_closed_at=ledger_closed_at,
            status=status,
            function_name=function_name,
            args_decoded=args_decoded,
            result_decoded=result_decoded,
            result_xdr=result_xdr,
            resource_fee_charged=resource_fee_charged,
            cpu_insn=cpu_insn,
            mem_byte=mem_byte,
            ledger_read_byte=ledger_read_byte,
            ledger_write_byte=ledger_write_byte,
            application_order=application_order,
            events=events,
            failure_category=failure_category,
            failure_code=failure_code,
        )

        invocation_detail.additional_properties = d
        return invocation_detail

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
