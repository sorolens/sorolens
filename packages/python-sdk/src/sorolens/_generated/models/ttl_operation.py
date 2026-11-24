from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.ttl_operation_network import TTLOperationNetwork
from ..models.ttl_operation_op_type import TTLOperationOpType
from ..types import UNSET, Unset

T = TypeVar("T", bound="TTLOperation")


@_attrs_define
class TTLOperation:
    """
    Attributes:
        id (str):
        contract_id (str):
        network (TTLOperationNetwork):
        tx_hash (str):
        ledger (int):
        ledger_closed_at (datetime.datetime):
        op_type (TTLOperationOpType):
        key_xdr (str):
        inserted_at (datetime.datetime):
        old_live_ledger (int | None | Unset):
        new_live_ledger (int | None | Unset):
    """

    id: str
    contract_id: str
    network: TTLOperationNetwork
    tx_hash: str
    ledger: int
    ledger_closed_at: datetime.datetime
    op_type: TTLOperationOpType
    key_xdr: str
    inserted_at: datetime.datetime
    old_live_ledger: int | None | Unset = UNSET
    new_live_ledger: int | None | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        contract_id = self.contract_id

        network = self.network.value

        tx_hash = self.tx_hash

        ledger = self.ledger

        ledger_closed_at = self.ledger_closed_at.isoformat()

        op_type = self.op_type.value

        key_xdr = self.key_xdr

        inserted_at = self.inserted_at.isoformat()

        old_live_ledger = self.old_live_ledger

        new_live_ledger = self.new_live_ledger

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
                "op_type": op_type,
                "key_xdr": key_xdr,
                "inserted_at": inserted_at,
            }
        )
        if old_live_ledger is not UNSET:
            field_dict["old_live_ledger"] = old_live_ledger
        if new_live_ledger is not UNSET:
            field_dict["new_live_ledger"] = new_live_ledger

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = d.pop("id")

        contract_id = d.pop("contract_id")

        network = TTLOperationNetwork(d.pop("network"))

        tx_hash = d.pop("tx_hash")

        ledger = d.pop("ledger")

        ledger_closed_at = datetime.datetime.fromisoformat(d.pop("ledger_closed_at"))

        op_type = TTLOperationOpType(d.pop("op_type"))

        key_xdr = d.pop("key_xdr")

        inserted_at = datetime.datetime.fromisoformat(d.pop("inserted_at"))

        old_live_ledger = d.pop("old_live_ledger", UNSET)

        new_live_ledger = d.pop("new_live_ledger", UNSET)

        ttl_operation = cls(
            id=id,
            contract_id=contract_id,
            network=network,
            tx_hash=tx_hash,
            ledger=ledger,
            ledger_closed_at=ledger_closed_at,
            op_type=op_type,
            key_xdr=key_xdr,
            inserted_at=inserted_at,
            old_live_ledger=old_live_ledger,
            new_live_ledger=new_live_ledger,
        )

        ttl_operation.additional_properties = d
        return ttl_operation

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
