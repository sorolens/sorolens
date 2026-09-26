from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.v2_storage_entry_durability import V2StorageEntryDurability
from ..models.v2_storage_entry_status import V2StorageEntryStatus

T = TypeVar("T", bound="V2StorageEntry")


@_attrs_define
class V2StorageEntry:
    """
    Attributes:
        contract_id (str):
        network (str):
        key_xdr (str):
        key_decoded (Any):
        value_xdr (str):
        value_decoded (Any):
        durability (V2StorageEntryDurability):
        live_until_ledger (int):
        last_modified_ledger (int):
        status (V2StorageEntryStatus):
        last_seen_at (datetime.datetime):
    """

    contract_id: str
    network: str
    key_xdr: str
    key_decoded: Any
    value_xdr: str
    value_decoded: Any
    durability: V2StorageEntryDurability
    live_until_ledger: int
    last_modified_ledger: int
    status: V2StorageEntryStatus
    last_seen_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        network = self.network

        key_xdr = self.key_xdr

        key_decoded = self.key_decoded

        value_xdr = self.value_xdr

        value_decoded = self.value_decoded

        durability = self.durability.value

        live_until_ledger = self.live_until_ledger

        last_modified_ledger = self.last_modified_ledger

        status = self.status.value

        last_seen_at = self.last_seen_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "network": network,
                "key_xdr": key_xdr,
                "key_decoded": key_decoded,
                "value_xdr": value_xdr,
                "value_decoded": value_decoded,
                "durability": durability,
                "live_until_ledger": live_until_ledger,
                "last_modified_ledger": last_modified_ledger,
                "status": status,
                "last_seen_at": last_seen_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        network = d.pop("network")

        key_xdr = d.pop("key_xdr")

        key_decoded = d.pop("key_decoded")

        value_xdr = d.pop("value_xdr")

        value_decoded = d.pop("value_decoded")

        durability = V2StorageEntryDurability(d.pop("durability"))

        live_until_ledger = d.pop("live_until_ledger")

        last_modified_ledger = d.pop("last_modified_ledger")

        status = V2StorageEntryStatus(d.pop("status"))

        last_seen_at = datetime.datetime.fromisoformat(d.pop("last_seen_at"))

        v2_storage_entry = cls(
            contract_id=contract_id,
            network=network,
            key_xdr=key_xdr,
            key_decoded=key_decoded,
            value_xdr=value_xdr,
            value_decoded=value_decoded,
            durability=durability,
            live_until_ledger=live_until_ledger,
            last_modified_ledger=last_modified_ledger,
            status=status,
            last_seen_at=last_seen_at,
        )

        v2_storage_entry.additional_properties = d
        return v2_storage_entry

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
