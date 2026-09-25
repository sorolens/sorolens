from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.invocation_args_decoded import InvocationArgsDecoded


T = TypeVar("T", bound="Invocation")


@_attrs_define
class Invocation:
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
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.invocation_args_decoded import (
            InvocationArgsDecoded,
        )

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

        invocation = cls(
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
        )

        invocation.additional_properties = d
        return invocation

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
