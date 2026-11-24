from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..types import UNSET, Unset

T = TypeVar("T", bound="ContractVersion")


@_attrs_define
class ContractVersion:
    """One Wasm hash a contract has run (issue

    Attributes:
        id (int):
        contract_id (str):
        wasm_hash (str): Wasm hash (hex).
        first_seen_ledger (int): Ledger at which this hash was first observed.
        recorded_at (datetime.datetime):
        tx_hash (str | Unset): Transaction that installed the hash, when known.
        verified_source_ref (str | Unset): Verified source reference, when the hash has been verified.
    """

    id: int
    contract_id: str
    wasm_hash: str
    first_seen_ledger: int
    recorded_at: datetime.datetime
    tx_hash: str | Unset = UNSET
    verified_source_ref: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        contract_id = self.contract_id

        wasm_hash = self.wasm_hash

        first_seen_ledger = self.first_seen_ledger

        recorded_at = self.recorded_at.isoformat()

        tx_hash = self.tx_hash

        verified_source_ref = self.verified_source_ref

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "contract_id": contract_id,
                "wasm_hash": wasm_hash,
                "first_seen_ledger": first_seen_ledger,
                "recorded_at": recorded_at,
            }
        )
        if tx_hash is not UNSET:
            field_dict["tx_hash"] = tx_hash
        if verified_source_ref is not UNSET:
            field_dict["verified_source_ref"] = verified_source_ref

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = d.pop("id")

        contract_id = d.pop("contract_id")

        wasm_hash = d.pop("wasm_hash")

        first_seen_ledger = d.pop("first_seen_ledger")

        recorded_at = datetime.datetime.fromisoformat(d.pop("recorded_at"))

        tx_hash = d.pop("tx_hash", UNSET)

        verified_source_ref = d.pop("verified_source_ref", UNSET)

        contract_version = cls(
            id=id,
            contract_id=contract_id,
            wasm_hash=wasm_hash,
            first_seen_ledger=first_seen_ledger,
            recorded_at=recorded_at,
            tx_hash=tx_hash,
            verified_source_ref=verified_source_ref,
        )

        contract_version.additional_properties = d
        return contract_version

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
