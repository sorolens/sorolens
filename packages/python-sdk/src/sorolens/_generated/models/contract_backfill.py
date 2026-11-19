from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="ContractBackfill")


@_attrs_define
class ContractBackfill:
    """Progress of the new-contract backfill, returned by the contract detail endpoint once the indexer has started it.
    Ledgers are scanned from newest_ledger down to oldest_ledger, and a restarted indexer resumes at next_ledger.

        Attributes:
            oldest_ledger (int): Oldest ledger the backfill will reach.
            newest_ledger (int): Chain head when the backfill started.
            next_ledger (int): Newest ledger not scanned yet. Below oldest_ledger once finished.
            ledgers_total (int):
            ledgers_done (int):
            percent (float): Scanned share of the range, from 0 to 100.
            started_at (datetime.datetime):
            updated_at (datetime.datetime):
            completed_at (datetime.datetime | None):
    """

    oldest_ledger: int
    newest_ledger: int
    next_ledger: int
    ledgers_total: int
    ledgers_done: int
    percent: float
    started_at: datetime.datetime
    updated_at: datetime.datetime
    completed_at: datetime.datetime | None
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        oldest_ledger = self.oldest_ledger

        newest_ledger = self.newest_ledger

        next_ledger = self.next_ledger

        ledgers_total = self.ledgers_total

        ledgers_done = self.ledgers_done

        percent = self.percent

        started_at = self.started_at.isoformat()

        updated_at = self.updated_at.isoformat()

        completed_at: None | str
        if isinstance(self.completed_at, datetime.datetime):
            completed_at = self.completed_at.isoformat()
        else:
            completed_at = self.completed_at

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "oldest_ledger": oldest_ledger,
                "newest_ledger": newest_ledger,
                "next_ledger": next_ledger,
                "ledgers_total": ledgers_total,
                "ledgers_done": ledgers_done,
                "percent": percent,
                "started_at": started_at,
                "updated_at": updated_at,
                "completed_at": completed_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        oldest_ledger = d.pop("oldest_ledger")

        newest_ledger = d.pop("newest_ledger")

        next_ledger = d.pop("next_ledger")

        ledgers_total = d.pop("ledgers_total")

        ledgers_done = d.pop("ledgers_done")

        percent = d.pop("percent")

        started_at = datetime.datetime.fromisoformat(d.pop("started_at"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updated_at"))

        def _parse_completed_at(data: object) -> datetime.datetime | None:
            if data is None:
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                completed_at_type_0 = datetime.datetime.fromisoformat(data)

                return completed_at_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None, data)

        completed_at = _parse_completed_at(d.pop("completed_at"))

        contract_backfill = cls(
            oldest_ledger=oldest_ledger,
            newest_ledger=newest_ledger,
            next_ledger=next_ledger,
            ledgers_total=ledgers_total,
            ledgers_done=ledgers_done,
            percent=percent,
            started_at=started_at,
            updated_at=updated_at,
            completed_at=completed_at,
        )

        contract_backfill.additional_properties = d
        return contract_backfill

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
