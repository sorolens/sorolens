from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast

if TYPE_CHECKING:
  from ..models.event import Event
  from ..models.storage_entry import StorageEntry





T = TypeVar("T", bound="ContractSnapshot")



@_attrs_define
class ContractSnapshot:
    """ 
        Attributes:
            contract_id (str):
            network (str):
            ledger (int):
            first_tracked_ledger (int):
            storage (list[StorageEntry]):
            last_event (Event):
     """

    contract_id: str
    network: str
    ledger: int
    first_tracked_ledger: int
    storage: list[StorageEntry]
    last_event: Event
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        from ..models.event import Event # noqa: PLC0415
        from ..models.storage_entry import StorageEntry # noqa: PLC0415
        contract_id = self.contract_id

        network = self.network

        ledger = self.ledger

        first_tracked_ledger = self.first_tracked_ledger

        storage = []
        for storage_item_data in self.storage:
            storage_item = storage_item_data.to_dict()
            storage.append(storage_item)



        last_event = self.last_event.to_dict()


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "contract_id": contract_id,
            "network": network,
            "ledger": ledger,
            "first_tracked_ledger": first_tracked_ledger,
            "storage": storage,
            "last_event": last_event,
        })

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.event import Event # noqa: PLC0415
        from ..models.storage_entry import StorageEntry # noqa: PLC0415
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        network = d.pop("network")

        ledger = d.pop("ledger")

        first_tracked_ledger = d.pop("first_tracked_ledger")

        storage = []
        _storage = d.pop("storage")
        for storage_item_data in (_storage):
            storage_item = StorageEntry.from_dict(storage_item_data)



            storage.append(storage_item)


        last_event = Event.from_dict(d.pop("last_event"))




        contract_snapshot = cls(
            contract_id=contract_id,
            network=network,
            ledger=ledger,
            first_tracked_ledger=first_tracked_ledger,
            storage=storage,
            last_event=last_event,
        )


        contract_snapshot.additional_properties = d
        return contract_snapshot

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
