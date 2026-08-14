import AccommodationRoomsDashboard from "@/components/accommodation/AccommodationRoomsDashboard";

export default async function AccommodationHotelPage({ params }: { params: Promise<{ hotelId: string }> }) {
  const { hotelId } = await params;
  return <AccommodationRoomsDashboard hotelId={hotelId} />;
}
