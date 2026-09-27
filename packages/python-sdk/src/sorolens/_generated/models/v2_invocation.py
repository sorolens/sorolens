from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.v2_invocation_status import V2InvocationStatus
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.v2_invocation_args_decoded_type_0 import V2InvocationArgsDecodedType0


T = TypeVar("T", bound="V2Invocation")


@_attrs_define
class V2Invocation:
    """
    Attributes:
        tx_hash (str):
        contract_id (str):
        network (str):
        ledger (int):
        ledger_closed_at (datetime.datetime):
        status (V2InvocationStatus):
        function_name (None | str):
        resource_fee_charged_stroops (int): Renamed from v1's unitless `resource_fee_charged`.
        cpu_instructions (int):
        memory_bytes (int):
        ledger_read_bytes (int):
        ledger_write_bytes (int):
        application_order (int):
        args_decoded (None | Unset | V2InvocationArgsDecodedType0):
        result_decoded (Any | Unset):
        result_xdr (str | Unset):
    """

    tx_hash: str
    contract_id: str
    network: str
    ledger: int
    ledger_closed_at: datetime.datetime
    status: V2InvocationStatus
    function_name: None | str
    resource_fee_charged_stroops: int
    cpu_instructions: int
    memory_bytes: int
    ledger_read_bytes: int
    ledger_write_bytes: int
    application_order: int
    args_decoded: None | Unset | V2InvocationArgsDecodedType0 = UNSET
    result_decoded: Any | Unset = UNSET
    result_xdr: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        from ..models.v2_invocation_args_decoded_type_0 import (
            V2InvocationArgsDecodedType0,
        )

        tx_hash = self.tx_hash

        contract_id = self.contract_id

        network = self.network

        ledger = self.ledger

        ledger_closed_at = self.ledger_closed_at.isoformat()

        status = self.status.value

        function_name: None | str
        function_name = self.function_name

        resource_fee_charged_stroops = self.resource_fee_charged_stroops

        cpu_instructions = self.cpu_instructions

        memory_bytes = self.memory_bytes

        ledger_read_bytes = self.ledger_read_bytes

        ledger_write_bytes = self.ledger_write_bytes

        application_order = self.application_order

        args_decoded: dict[str, Any] | None | Unset
        if isinstance(self.args_decoded, Unset):
            args_decoded = UNSET
        elif isinstance(self.args_decoded, V2InvocationArgsDecodedType0):
            args_decoded = self.args_decoded.to_dict()
        else:
            args_decoded = self.args_decoded

        result_decoded = self.result_decoded

        result_xdr = self.result_xdr

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
                "resource_fee_charged_stroops": resource_fee_charged_stroops,
                "cpu_instructions": cpu_instructions,
                "memory_bytes": memory_bytes,
                "ledger_read_bytes": ledger_read_bytes,
                "ledger_write_bytes": ledger_write_bytes,
                "application_order": application_order,
            }
        )
        if args_decoded is not UNSET:
            field_dict["args_decoded"] = args_decoded
        if result_decoded is not UNSET:
            field_dict["result_decoded"] = result_decoded
        if result_xdr is not UNSET:
            field_dict["result_xdr"] = result_xdr

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.v2_invocation_args_decoded_type_0 import (
            V2InvocationArgsDecodedType0,
        )

        d = dict(src_dict)
        tx_hash = d.pop("tx_hash")

        contract_id = d.pop("contract_id")

        network = d.pop("network")

        ledger = d.pop("ledger")

        ledger_closed_at = datetime.datetime.fromisoformat(d.pop("ledger_closed_at"))

        status = V2InvocationStatus(d.pop("status"))

        def _parse_function_name(data: object) -> None | str:
            if data is None:
                return data
            return cast(None | str, data)

        function_name = _parse_function_name(d.pop("function_name"))

        resource_fee_charged_stroops = d.pop("resource_fee_charged_stroops")

        cpu_instructions = d.pop("cpu_instructions")

        memory_bytes = d.pop("memory_bytes")

        ledger_read_bytes = d.pop("ledger_read_bytes")

        ledger_write_bytes = d.pop("ledger_write_bytes")

        application_order = d.pop("application_order")

        def _parse_args_decoded(
            data: object,
        ) -> None | Unset | V2InvocationArgsDecodedType0:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            try:
                if not isinstance(data, dict):
                    raise TypeError()
                args_decoded_type_0 = V2InvocationArgsDecodedType0.from_dict(data)

                return args_decoded_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(None | Unset | V2InvocationArgsDecodedType0, data)

        args_decoded = _parse_args_decoded(d.pop("args_decoded", UNSET))

        result_decoded = d.pop("result_decoded", UNSET)

        result_xdr = d.pop("result_xdr", UNSET)

        v2_invocation = cls(
            tx_hash=tx_hash,
            contract_id=contract_id,
            network=network,
            ledger=ledger,
            ledger_closed_at=ledger_closed_at,
            status=status,
            function_name=function_name,
            resource_fee_charged_stroops=resource_fee_charged_stroops,
            cpu_instructions=cpu_instructions,
            memory_bytes=memory_bytes,
            ledger_read_bytes=ledger_read_bytes,
            ledger_write_bytes=ledger_write_bytes,
            application_order=application_order,
            args_decoded=args_decoded,
            result_decoded=result_decoded,
            result_xdr=result_xdr,
        )

        v2_invocation.additional_properties = d
        return v2_invocation

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
