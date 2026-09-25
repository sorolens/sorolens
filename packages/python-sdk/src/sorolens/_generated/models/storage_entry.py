from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast
import datetime






T = TypeVar("T", bound="StorageEntry")



@_attrs_define
class StorageEntry:
    """ 
        Attributes:
            contract_id (str):
            network (str):
            key_xdr (str):
            value_xdr (str):
            durability (str):
            live_until_ledger (int):
            last_modified_ledger (int):
            status (str):
            last_seen_at (datetime.datetime):
            key_decoded (Any | Unset):
            value_decoded (Any | Unset):
     """

    contract_id: str
    network: str
    key_xdr: str
    value_xdr: str
    durability: str
    live_until_ledger: int
    last_modified_ledger: int
    status: str
    last_seen_at: datetime.datetime
    key_decoded: Any | Unset = UNSET
    value_decoded: Any | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        network = self.network

        key_xdr = self.key_xdr

        value_xdr = self.value_xdr

        durability = self.durability

        live_until_ledger = self.live_until_ledger

        last_modified_ledger = self.last_modified_ledger

        status = self.status

        last_seen_at = self.last_seen_at.isoformat()

        key_decoded = self.key_decoded

        value_decoded = self.value_decoded


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "contract_id": contract_id,
            "network": network,
            "key_xdr": key_xdr,
            "value_xdr": value_xdr,
            "durability": durability,
            "live_until_ledger": live_until_ledger,
            "last_modified_ledger": last_modified_ledger,
            "status": status,
            "last_seen_at": last_seen_at,
        })
        if key_decoded is not UNSET:
            field_dict["key_decoded"] = key_decoded
        if value_decoded is not UNSET:
            field_dict["value_decoded"] = value_decoded

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        network = d.pop("network")

        key_xdr = d.pop("key_xdr")

        value_xdr = d.pop("value_xdr")

        durability = d.pop("durability")

        live_until_ledger = d.pop("live_until_ledger")

        last_modified_ledger = d.pop("last_modified_ledger")

        status = d.pop("status")

        last_seen_at = datetime.datetime.fromisoformat(d.pop("last_seen_at"))




        key_decoded = d.pop("key_decoded", UNSET)

        value_decoded = d.pop("value_decoded", UNSET)

        storage_entry = cls(
            contract_id=contract_id,
            network=network,
            key_xdr=key_xdr,
            value_xdr=value_xdr,
            durability=durability,
            live_until_ledger=live_until_ledger,
            last_modified_ledger=last_modified_ledger,
            status=status,
            last_seen_at=last_seen_at,
            key_decoded=key_decoded,
            value_decoded=value_decoded,
        )


        storage_entry.additional_properties = d
        return storage_entry

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
