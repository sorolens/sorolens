from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.contract import Contract
    from ..models.contract_snapshot_export_summary import ContractSnapshotExportSummary
    from ..models.event import Event
    from ..models.storage_entry import StorageEntry


T = TypeVar("T", bound="ContractSnapshotExport")


@_attrs_define
class ContractSnapshotExport:
    """
    Attributes:
        schema_version (int): Version of the export shape; currently 1.
        contract_id (str):
        network (str):
        ledger (int): Newest indexed ledger the export is keyed to.
        metadata (Contract):
        storage (list[StorageEntry]): Live storage entries, one per key, sorted by key_xdr.
        events (list[Event]): Most recent events, newest first (up to 50).
        summary (ContractSnapshotExportSummary):
    """

    schema_version: int
    contract_id: str
    network: str
    ledger: int
    metadata: Contract
    storage: list[StorageEntry]
    events: list[Event]
    summary: ContractSnapshotExportSummary
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        schema_version = self.schema_version

        contract_id = self.contract_id

        network = self.network

        ledger = self.ledger

        metadata = self.metadata.to_dict()

        storage = []
        for storage_item_data in self.storage:
            storage_item = storage_item_data.to_dict()
            storage.append(storage_item)

        events = []
        for events_item_data in self.events:
            events_item = events_item_data.to_dict()
            events.append(events_item)

        summary = self.summary.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "schema_version": schema_version,
                "contract_id": contract_id,
                "network": network,
                "ledger": ledger,
                "metadata": metadata,
                "storage": storage,
                "events": events,
                "summary": summary,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.contract import Contract
        from ..models.contract_snapshot_export_summary import (
            ContractSnapshotExportSummary,
        )
        from ..models.event import Event
        from ..models.storage_entry import StorageEntry

        d = dict(src_dict)
        schema_version = d.pop("schema_version")

        contract_id = d.pop("contract_id")

        network = d.pop("network")

        ledger = d.pop("ledger")

        metadata = Contract.from_dict(d.pop("metadata"))

        storage = []
        _storage = d.pop("storage")
        for storage_item_data in _storage:
            storage_item = StorageEntry.from_dict(storage_item_data)

            storage.append(storage_item)

        events = []
        _events = d.pop("events")
        for events_item_data in _events:
            events_item = Event.from_dict(events_item_data)

            events.append(events_item)

        summary = ContractSnapshotExportSummary.from_dict(d.pop("summary"))

        contract_snapshot_export = cls(
            schema_version=schema_version,
            contract_id=contract_id,
            network=network,
            ledger=ledger,
            metadata=metadata,
            storage=storage,
            events=events,
            summary=summary,
        )

        contract_snapshot_export.additional_properties = d
        return contract_snapshot_export

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
