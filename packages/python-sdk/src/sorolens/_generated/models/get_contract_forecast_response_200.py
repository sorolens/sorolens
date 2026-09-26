from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.forecast_series import ForecastSeries


T = TypeVar("T", bound="GetContractForecastResponse200")


@_attrs_define
class GetContractForecastResponse200:
    """
    Attributes:
        contract_id (str):
        lookback_days (int):
        series (list[ForecastSeries]):
    """

    contract_id: str
    lookback_days: int
    series: list[ForecastSeries]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        lookback_days = self.lookback_days

        series = []
        for series_item_data in self.series:
            series_item = series_item_data.to_dict()
            series.append(series_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "lookback_days": lookback_days,
                "series": series,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.forecast_series import ForecastSeries

        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        lookback_days = d.pop("lookback_days")

        series = []
        _series = d.pop("series")
        for series_item_data in _series:
            series_item = ForecastSeries.from_dict(series_item_data)

            series.append(series_item)

        get_contract_forecast_response_200 = cls(
            contract_id=contract_id,
            lookback_days=lookback_days,
            series=series,
        )

        get_contract_forecast_response_200.additional_properties = d
        return get_contract_forecast_response_200

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
