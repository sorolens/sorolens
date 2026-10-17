from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.contract_note import ContractNote


T = TypeVar("T", bound="ContractNoteList")


@_attrs_define
class ContractNoteList:
    """
    Attributes:
        contract_id (str):
        notes (list[ContractNote]):
    """

    contract_id: str
    notes: list[ContractNote]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        notes = []
        for notes_item_data in self.notes:
            notes_item = notes_item_data.to_dict()
            notes.append(notes_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "notes": notes,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.contract_note import ContractNote

        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        notes = []
        _notes = d.pop("notes")
        for notes_item_data in _notes:
            notes_item = ContractNote.from_dict(notes_item_data)

            notes.append(notes_item)

        contract_note_list = cls(
            contract_id=contract_id,
            notes=notes,
        )

        contract_note_list.additional_properties = d
        return contract_note_list

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
